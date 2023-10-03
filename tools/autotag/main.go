package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

type TagOptions struct {
	Push bool

	// ExtraGitTags controls the behaviour when extra git tags are found
	ExtraGitTags string

	// MissingGitTags controls the behaviour when desired tags do not exist in git
	MissingGitTags string
}

func (o *TagOptions) InitDefaults() {
	o.Push = false
	o.ExtraGitTags = "error"
	o.MissingGitTags = "add"
}

func run(ctx context.Context) error {
	var opt TagOptions
	opt.InitDefaults()

	tagsFile := ".tags"
	flag.StringVar(&tagsFile, "tags", tagsFile, "file containing tags")

	flag.BoolVar(&opt.Push, "push", opt.Push, "push new tags to upstream")
	flag.StringVar(&opt.ExtraGitTags, "extra-git-tags", opt.ExtraGitTags, "behaviour when git tags are found that are not in desired list")
	flag.StringVar(&opt.MissingGitTags, "missing-git-tags", opt.MissingGitTags, "behaviour when desired tags not found in git")

	klog.InitFlags(nil)

	flag.Parse()

	desiredTags, err := parseTagsFile(ctx, tagsFile)
	if err != nil {
		return err
	}

	desiredTagsMap, err := buildTagMap(desiredTags)
	if err != nil {
		return err
	}

	gitTags, err := getGitTags(ctx)
	if err != nil {
		return err
	}

	gitTagsMap, err := buildTagMap(gitTags)
	if err != nil {
		return err
	}

	var errs []error

	var addTags []Tag

	for _, gitTag := range gitTagsMap {
		_, exists := desiredTagsMap[gitTag.Tag]

		switch opt.ExtraGitTags {
		case "error":
			if !exists {
				errs = append(errs, fmt.Errorf("tag %q found in git but not in desired tags", gitTag.Tag))
			}
		case "warn":
			if !exists {
				klog.Warningf("tag %q found in git but not in desired tags", gitTag.Tag)
			}

		default:
			errs = append(errs, fmt.Errorf("invalid value for --extra-git-tags %q", opt.ExtraGitTags))
		}
	}

	for _, tag := range desiredTagsMap {
		gitTag, exists := gitTagsMap[tag.Tag]
		if exists {
			if tag.SHA != gitTag.SHA {
				errs = append(errs, fmt.Errorf("tag %q does not match; git has %v, desired is %v", tag.Tag, gitTag.SHA, tag.SHA))
			} else {
				klog.V(2).Infof("tag %v matches", tag)
			}
		} else {
			switch opt.MissingGitTags {
			case "add":
				addTags = append(addTags, tag)
			case "warn":
				klog.Warningf("tag %q found in desired tags but not in git", tag.Tag)

			default:
				errs = append(errs, fmt.Errorf("invalid value for --missing-git-tags %q", opt.MissingGitTags))
			}
		}
	}

	if len(errs) == 0 {
		if len(addTags) != 0 {
			for _, addTag := range addTags {
				if err := addAndPushTag(ctx, addTag, opt); err != nil {
					return err
				}
			}
		}
	}

	return errors.Join(errs...)
}

func addAndPushTag(ctx context.Context, tag Tag, opt TagOptions) error {
	klog.Infof("adding tag %+v", tag)
	args := []string{"tag"}
	// Create an annotated tag
	args = append(args, "-a", "-m", "Tag "+tag.Tag)
	args = append(args, tag.Tag, tag.SHA)

	if _, err := runGit(ctx, args...); err != nil {
		return err
	}

	if opt.Push {
		klog.Infof("pushing tag %+v", tag)
		if _, err := runGit(ctx, "push", "origin", tag.Tag); err != nil {
			return err
		}
	} else {
		klog.Infof("--push not set, skipping push")
	}

	return nil
}

type Tag struct {
	SHA string
	Tag string
}

func buildTagMap(tags []Tag) (map[string]Tag, error) {
	m := make(map[string]Tag)
	for _, tag := range tags {
		if _, exists := m[tag.Tag]; exists {
			return nil, fmt.Errorf("duplicate tag %q found", tag.Tag)
		}
		m[tag.Tag] = tag
	}
	return m, nil
}

func runGit(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	klog.V(1).Infof("running git command: %q", strings.Join(cmd.Args, " "))

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "stdout: %v\n", stdout.String())
		fmt.Fprintf(os.Stderr, "stderr: %v\n", stderr.String())
		return nil, fmt.Errorf("error running command %q: %w", strings.Join(cmd.Args, " "), err)
	}

	b := stdout.Bytes()
	return b, nil
}

func getGitTags(ctx context.Context) ([]Tag, error) {
	b, err := runGit(ctx, "show-ref", "--tags")
	if err != nil {
		return nil, err
	}

	tags, err := parseTags(b)
	if err != nil {
		return nil, fmt.Errorf("parsing tags from git: %w", err)
	}
	for i := range tags {
		tag := &tags[i]
		tag.Tag = strings.TrimPrefix(tag.Tag, "refs/tags/")
	}
	return tags, nil
}

func parseTagsFile(ctx context.Context, p string) ([]Tag, error) {
	klog.V(1).Infof("parsing tags file %q", p)
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("opening file %q: %w", p, err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading file %q: %w", p, err)
	}

	tags, err := parseTags(b)
	if err != nil {
		return nil, fmt.Errorf("parsing tags from file %q: %w", p, err)
	}
	return tags, nil
}

func parseTags(b []byte) ([]Tag, error) {
	var tags []Tag
	r := bufio.NewReader(bytes.NewReader(b))
	done := false
	for !done {
		s, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				done = true
			} else {
				return nil, fmt.Errorf("reading line: %w", err)
			}
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "#") {
			continue
		}
		tokens := strings.Fields(s)
		if len(tokens) != 2 {
			return nil, fmt.Errorf("unexpected line %q (expected two tokens)", s)
		}
		tags = append(tags, Tag{
			SHA: tokens[0],
			Tag: tokens[1],
		})
	}
	return tags, nil
}
