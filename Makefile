# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.EXPORT_ALL_VARIABLES:
OUT_DIR ?= _output
BIN_DIR := $(OUT_DIR)/bin

.PHONY: all
all: test bin images

.PHONY: clean
clean:
	rm -rf _output

.PHONY: test
test:
	./build/run-tests.sh

.PHONY: bin
bin:
	./build/build-bin.sh

.PHONY: images
images: bin
	./build/build-images.sh

.PHONY: release-tars
release-tars: bin images
	./build/build-release-tars.sh
