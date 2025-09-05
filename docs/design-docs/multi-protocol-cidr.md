## Native MixedProtocolLBService Support in the GKE Service Controller

### **1\. Abstract**

This document proposes a design to implement support for Kubernetes Service objects of `type: LoadBalancer`
with mixed protocols (e.g., TCP and UDP) in the cloud-provider-gcp controller.
The core of this proposal is to evolve the controller's logic from managing a
single GCP Forwarding Rule per Kubernetes Service to managing a set of forwarding rules,
one for each protocol specified in the Service manifest.
This change will align GCP's behavior with the generally available `MixedProtocolLBService`
feature in upstream Kubernetes, eliminating the need for the current "dual-service, single-IP" workaround.

### **2\. Motivation**

The `MixedProtocolLBService` feature gate has been stable in Kubernetes since v1.26, allowing users to define a single LoadBalancer Service with ports for both TCP and UDP.
However, the GCP Service controller does not currently support this.
The controller's logic is based on a one-to-one mapping between a Service object
and a single GCP Forwarding Rule.
Since a GCP Forwarding Rule is inherently bound to a single protocol (TCP or UDP), the controller cannot provision the necessary infrastructure for a mixed-protocol service.

This forces users to adopt a less intuitive workaround:
deploying two separate Service objects (one for TCP, one for UDP)
and manually assigning them to the same reserved static IP address.
While functional, this approach is cumbersome, increases configuration complexity,
and is not aligned with the declarative intent of the Kubernetes API.

Implementing `MixedProtocolLBService` support would provide a superior user experience,
reduce configuration errors,
and make GCP's networking capabilities consistent with the core Kubernetes feature set here.

### **3\. Proposed Design**

The proposed design modifies the reconciliation loop within the GCP Service controller
to manage a collection of forwarding rules for each LoadBalancer Service, rather than just one.

#### **3.1. Core Controller Logic Modification**

The primary changes will be within the `EnsureLoadBalancer` method in the GCP cloud provider's Service controller. The existing logic creates a single load balancer configuration. The new logic will perform the following steps during its reconciliation loop:

1. **Protocol Grouping:** Upon receiving a Service object, the controller will first inspect the `spec.ports` array and group the ports by their declared protocol (e.g., create a map of `corev1.Protocol` -> `corev1.ServicePort`).  
2. **IP Address Management:**  
   * If a static IP is specified in `spec.loadBalancerIP`, it will be used for all forwarding rules.  
   * If no IP is specified, the controller will reserve a new static IP address upon the first reconciliation. This address will be used for all forwarding rules created for this Service. The controller must ensure this IP is retained across updates and released only upon Service deletion.  
3. **Per-Protocol Reconciliation Loop:** The controller will iterate through the grouped protocols. For each protocol (e.g., TCP, UDP):  
   * **Generate Desired State:** It will construct a set of desired GCP Forwarding Rule object.  
     * **Naming Convention:** To avoid collisions and maintain a clear association, forwarding rules will be named using a convention that includes the service UID and the protocol. A proposed convention is `k8s-fw-\[service-uid\]-\[protocol\]`, for example, `k8s-fw-a8b4f12-tcp`.  
     * **Configuration:** Each forwarding rule will be configured with the shared static IP, the specific protocol (TCP or UDP), and the list of ports for that protocol.  
   * **Reconcile with Actual State:** The controller will check if a forwarding rule with the generated name already exists in GCP.  
     * **Create:** If the rule does not exist, it will be created.  
     * **Update:** If the rule exists, its configuration will be compared against the desired state and updated if necessary.  
     * **No-Op:** If the existing rule matches the desired state, no action is taken.  
4. **Resource Garbage Collection:** After the reconciliation loop for all protocols present in the Service spec, the controller must clean up any orphaned resources. It will list all GCP Forwarding Rules that match the Service's UID pattern (`k8s-fw-\[service-uid\]-\*`). If any of these existing rules correspond to a protocol that has been removed from the Service spec, the controller will delete that now-orphaned forwarding rule.  
5. **Backend Resource Management:** The underlying GCP Backend Service or Target Pool can typically be shared across the different forwarding rules, as it defines the set of backend nodes, not the frontend protocol. The controller logic for managing the backend service will largely remain the same, ensuring it points to the correct set of cluster nodes.

#### **3.2. Deletion Logic Modification**

The `EnsureLoadBalancerDeleted` method must also be updated. When a Service is deleted, the controller will use the naming convention (`k8s-fw-\[service-uid\]-\*`) to find and delete *all* associated forwarding rules, in addition to the backend service and the reserved static IP address (if it was provisioned by the controller).

### **4\. Code Implementation Pointers**

The necessary changes would be localized within the GCP-specific implementation of the cloudprovider.LoadBalancer interface.7

* **Primary Files for Modification:** The core logic for L4 load balancer reconciliation is located in the gce package. The files managing Service objects of `type: LoadBalancer` would be the main focus.  
* **Key Functions:**  
  * gce.EnsureLoadBalancer: This function would need to be refactored to contain the protocol-grouping and per-protocol reconciliation loop described above.  
  * gce.EnsureLoadBalancerDeleted: This function would need to be updated to iterate through all potential forwarding rules based on the new naming scheme and delete them.  
  * **Resource Naming Functions:** Helper functions that generate names for GCP resources would need to be adapted to produce the protocol-specific forwarding rule names.

### 5. Production Readiness

For ease of introduction, we will implement a feature flag for the support.  If the feature flag is not set,
the existing behaviour will be used - specifying a service with multiple protocols will be an error.

So that traffic is not interrupted, if a ForwardingRule exists with the "old" naming convention (`k8s-fw-\[service-uid\]`),
that name will be used for the matching desired ForwardingRule.

### **5\. Alternatives Considered**

* **Continue with Dual-Service Workaround:** This is the current state. It is not a true implementation of the Kubernetes feature and places an unnecessary operational burden on users.  
* **Rely on Gateway API:** The Gateway API is the long-term strategic direction for advanced Kubernetes networking.10 However, it is a separate, more complex API. This proposal aims to fix a specific deficiency in the existing and widely used  
  Service API, which should function as specified by the core Kubernetes project. Implementing this feature does not preclude or conflict with the ongoing development of the Gateway API.

By adopting this design, the GCP Service controller can provide a seamless and intuitive experience for users needing to expose mixed-protocol applications, directly translating the Kubernetes API's intent into the necessary underlying cloud infrastructure.


