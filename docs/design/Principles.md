## PCS Design Philosophy & Principles

### Provenance

PCS must be the preeminent maintainer of power state information. 

In Shasta today, Hardware State Manager keeps several state flags, one of which keeps a hybrid of power state; It reports `on`, `off`, `standby`, and `ready`, etc. This has caused confusion as `on` and `off` are hardware states, whereas `ready` is a logical state that implies the power is on. Furthermore some APIs in Shasta CAPMC report power state by querying the redfish devices whereas other APIs query the HSM for power state.  This ultimately leads to confusion as those states can be  unreconciled due to background notification channels failing. 

PCS should keep records of the HARDWARE states; not the HSM states. In today's Shasta v1.4+ (CSM v1.0+) architecture HSM gets a message over Kafka bus for on/off; to know if things actually turned on or off; ready and standby are from the heartbeat demon. PCS should become the authority on power state, which is a break from current Shasta design but necessary to alleviate confusion. PCS should listen for redfish events (which HSM does currently); and be THE source of truth for all other services, including HSM regarding PHYSICAL power state.  PCS should not be the keeper of logical states like 'ready' or 'standby'; that should remain with HSM.


### Identity
The primary hardware identifier of the system (PCS) is the xname; some entities really like to use NID; but that is an alias, and not everything has a nid (eg BMCs). PCS must use xnames, we need to decide what level of support we give to alternative descriptions of xnames (nids, partitions, groups), but Xname is the identifier. An additional reason for this focus on xname is that the locking and reservation API (part of hardware state manager) can only operate on xnames; groups, partitions will not work (reservations are unique per xname). Finally, because of our multi-tenancy story nids are guaranteed to not be unique across tenants, so xnames must be used.

### Set & Hierarchies
Fundamentally many functions that PCS performs (power transitions, power capping) should be applied to a set of components. To accomplish scalability we need the caller to be able to specify a set of components. Tangential to set notation is the notion of a power hierarchy (e.g. `a` is the child of `b`). 

### Clear system boundaries
Shasta CAPMC has endpoints and resources for several other systems (eg state manager data, telemetry data) because it was developed following the monolithic architecture of Cascade CAPMC.  PCS must not present other services data unless it is directly beneficial to PCS.  I.E. PCS will not contain a node-nid map; that is HSM functionality, and callers must go to HSM to get that data.  We should not have APIs that shadow other microservices.

### Separation of Concerns
The PCS API will be developed independently from the PCS CLI.  For a variety of reasons the CSM API and CLI's have been too tightly coupled such that CLI considerations have needlessly constrained the API design.  We will embrace the CLI as a first class citizen, recognizing that this will require a greater level of effort than relying on the generator, but will lead to a greater outcome. CLIs are for humans first; APIs are for automation first.  This recognition of responsibility will help us to put the right level of abstraction and logic in each tool.

### Robustness
PCS must always take a 'best effort' approach to management across components because:  

1. the network is unreliable (as a general rule of distributed systems)
2. the hardware devices are unreliable (as system scale increases MTBF decreases [Failure Analysis and Quantification for Contemporary and Future Supercomputers](https://arxiv.org/pdf/1911.02118.pdf))

Therefore any operation that must execute across ALL components as a condition for success is unlikely to ever succeed. Furthermore as the operations undertaken by PCS are based in the physical domain, the ability to 'rollback' is severely limited.  e.g. you cannot 'undo' a power transition; you can do a different power transition and get the component back to the previous state, but that is a new transition.

Therefore for almost every action of PCS that 'does' something, vs 'tells' us something, we must always cope with the very real likelihood that some subset of the components will fail to accomplish. The likelihood of partial failure necessitates informative and useful error messages.

### Vendor Agnosticism
Redfish is the non-standard-standard. It's a great tool; but the vendor differences or even model differences make management of hardware challenging.  PCS will present an abstraction layer for power control.  PCS will do its best to use un-ambiguous language, but PCS ultimately will own the abstraction between the management software and the underlying hardware.  We are going to 'abstract' away the redfish implementations from the API and present our own unified control layer. This does mean that PCS will have to be very redfish dialect fluent; but this is the right place for this type of abstraction to live.  This will decrease the callers need to have vendor dependent knowledge, which will increase usability and visibility of the system. 