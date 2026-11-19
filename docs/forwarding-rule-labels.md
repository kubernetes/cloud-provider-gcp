# Regional forwarding-rule labels

The legacy GCE cloud-controller-manager can manage labels on the regional
forwarding rule for an internal or external LoadBalancer Service. This does not
label other load-balancer resources or enable support in other LB controllers.

Operators must enable `--enable-forwarding-rule-labels` (default: `false`). When
disabled, CCM ignores both label annotations, including malformed values, and
leaves existing GCP labels and ownership bookkeeping unchanged.

Configure desired labels on the Service:

```yaml
metadata:
  annotations:
    cloud.google.com/load-balancer-forwarding-rule-labels: "env=prod,team=payments"
```

The value is a comma-separated list of `key=value` pairs satisfying GCP label
requirements. Empty label values (`key=`) are valid. The previous unreleased
`cloud.google.com/load-balancer-resource-labels` name is not supported.

## Ownership and conflicts

CCM preserves unrelated labels already on the forwarding rule. Adding a key to
the desired annotation claims ownership of that key: the configured value wins
over any existing or externally changed value. External systems must not manage
the same keys concurrently. CCM does not save or restore overwritten values.

Removing a key removes that label from GCP. An empty desired annotation, or
removing the desired annotation entirely, removes only CCM-managed labels and
preserves unrelated labels. Disabling the flag instead stops label management
without cleaning up labels.

CCM maintains the following annotation on the **Service**:

`cloud.google.com/managed-load-balancer-forwarding-rule-label-keys`

This is a sorted, comma-separated list of keys owned (or potentially applied) by
CCM, not a second user input. Users and other controllers, including HyperShift,
must update only the desired-label annotation and leave this bookkeeping intact.
Deleting or editing the bookkeeping can prevent cleanup or incorrectly claim
external keys. CCM records new keys before cloud writes and retains withdrawn
keys until a successful cloud update, allowing retries after partial failures.

Updates use the forwarding rule's label fingerprint. Concurrent cloud changes
can cause reconciliation to fail and retry; labels are merged again on retry.
The GCP limit of 64 labels applies to the combined managed and unrelated labels.
Label errors are returned after required network/firewall reconciliation.

CCM requires permission to patch Service metadata and
`compute.forwardingRules.setLabels` in addition to existing LB permissions.
