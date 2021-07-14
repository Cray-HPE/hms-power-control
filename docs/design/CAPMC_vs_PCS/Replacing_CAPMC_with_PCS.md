# Future of Power Control in Shasta

*Andrew Nieuwsma | June 2021*

## A **VERY** brief overview of CAPMC

CAPMC - Cray Advanced Platform Monitoring and Control,  is an API initially directed "toward workload manager vendors to enable power-aware scheduling and resource management on Cray XC-series systems and beyond" [CAPMC](https://cug.org/proceedings/cug2015_proceedings/includes/files/pap132.pdf). Cascade CAPMC was both an API and a CLI.  `capmc` was the CLI, `CAPMC` was the API.  PCS, like Shasta CAPMC, will have both an API and a CLI.

CAPMC traditionally has focused on two major actors: work load managers (WLMs) and admins.  The seminal paper on [CAPMC](https://cug.org/proceedings/cug2015_proceedings/includes/files/pap132.pdf) describes the core use cases of CAPMC:

 * system power monitoring
 * node power/energy monitoring
 * node power on/off control
 * power capping control
 * power aware scheduling of an over provisioned system

The secondary use cases (as determined by contrasting the API of XC with the aforementioned white paper) includes:

 * power biasing
 * KNL support (mcdram control)
 * SSD control

Power control (i.e. powering devices on or off) is an important part of the CAPMC portfolio, but is not in any way to be confused with the full CAPMC purpose.   CAPMC in XC was functionally a monolith, as were many parts of the XC architecture.  Shasta CAPMC was built on the assumption of a system architecture similar to Cascade (XC).  Shasta CAPMC was a jumping off point, our first major service to do power control in Shasta. It inherited several of the 'monolith' patterns that XC CAPMC had and are not best suited for the microservices architecture of Cray Systems Management. 

Shasta CAPMC looks different than its XC predecessor.  Shasta CAPMC had no need to ever consider the secondary use cases as they were not requirements for Shasta. 

Over several years of development, integration, customer and developer feedback we identified the need for a new approach.  This new approach has led to the conception of a purposeful split of the monolith and re-implementation of power control, hence  `PCS - Power Control Service`. 

## Why re-implement, re-design CAPMC?

There are several core reasons why we are re-working our approach to power control.

### architectural alignment (micro-service) 
Cray Systems Management has a much more solidified Shasta architecture and we have much more mature set of capabilities for developing RESTful microservices. CAPMC is very monolithic and has a very large portfolio.  As the Shasta product has many capabilities already provided across the platform (e.g. telemetry  via SMF, or p-state control on node) it makes sense to leverage those capabilities where they are most mature.

### core functionality (resolving power states) 
Perhaps the most common complaint of CAPMC is that when someone uses it to power on/off a node that CAPMC 'tries' and gives up; often times without returning useful information or cryptic information at best.  CAPMC is a hardware abstraction but purposefully does not cope with the reliability levels of the hardware. A try-once-and-die model will not scale as we approach exascale and wont help us meet our SLOs (system level objectives) around booting. CAPMC cannot be modified to better cope with this reality of hardware as this was the intentional paradigm of the API.  

CAPMC is approximately (with regards to powering on and off of devices) about one step more advanced and abstracted than just using redfish. Meaning that it 'abstracts' most of the uniquenesses of the individual devices and can cope to some degree with their irregularity. A higher level of abstraction will further separate the caller from redfish and the 'base' idea of just turning on or off a device, but working towards guaranteeing that a device experiences its intended power transition. An even higher level of abstraction would introduce the idea of coordination across devices (orchestration).  

### towards 'ramp rate limiting'
A feature that Shasta CAPMC does not yet have, is some notion of 'ramp rate limiting'. Im layman's terms, controlling the amount of devices you power on (or off) to not exceed a rate of change. This is important for customers in power sensitive markets and can many times be tied to system acceptance.  PCS will need to share state across instances to know how many devices are being powered up at a time to stay within an envelop. This feature set is not trivial and the programming necessary is not very doable given the level of abstraction that CAPMC offers from Redfish.   

### maintainability
Shasta CAPMC does not have a well layered design. The design is very vertically siloed. This makes resolving power states and working towards 'ramp rate limiting' very difficult. The code has fairly mature unit testing, but because of the lack of horizontal break points in the design any testing requires mocking a full http stack for every call (more or less). This means that unit tests aren't testing 'units' of code but more 'features'.


## Splitting the Monolith

Shasta CAPMC had a lot of features that PCS will not recreate or carry on.  They will be discontinued from the 'power control' portfolio because there are other more appropriate ways to engage with the resources.  

The features of the power control service are limited to (As of 2021Q3 plan):

* powering of devices  (`transitions`)
* reporting power status of devices (`power-status`)
* power capping (`power-cap`)

There are many ways to think about what we've split out of CAPMC, in terms of use cases, features, APIs, larger Cray system objectives, etc.   I will attempt to distill the information in a few different views; use the one most relevant to your stakeholder class.


### Use Cases

This table shows how the original CAPMC primary use cases are addressed:

| Shasta CAPMC | PCS | Explanation |
| --- | --- | --- |
| system power monitoring | -- | split out from PCS, All environmental telemetery is part of the Shasta Monitoring Framework. SMF has the right tools and capabilities to expose the data (SQL, grafana, etc) for customer use cases. |
| node power/energy monitoring | -- | split out from PCS, All environmental telemetery is part of the Shasta Monitoring Framework. SMF has the right tools and capabilities to expose the data (SQL, grafana, etc) for customer use cases. |
| node power on/off control | component power on/off control | PCS expands power controls beyond 'compute nodes' to components more generally. This is core functionality of PCS. |
| power capping control | power capping control | No functional difference; API differences, but same capabilities. |
| power aware scheduling of an over provisioned system | *depends | This use case is a composite of many features of CAPMC + WLM integration.  We still plan and expect WLM integration. From our discussions we expect that integration to revolve mostly around power capping and power control (telemetry information will be gathered directly from on-node sources and is not the purview of PCS, or the Cray Systems Managment services).

### Features

From the five use cases we expand to features that embody them.  I will represent this as a pseudo listing of the CLI commands as I think that will be the clearest way to make sense of the features currently.  Several of the Shasta CAPMC features have been coalesces or remapped into new features in PCS. All API calls and CLI calls will be different between CAPMC and PCS. It is VERY important that you do not read this as a list of code functions or API paths, they are not! This is a 'human friendly' list of features.

| Shasta CAPMC | PCS Mapping| Explanation |
| --- | --- | --- |
|  emergency power off | emergency power off | administrators can do forced, fast, power off through transitions API.|
|  get power cap | get power cap | |
|  get power cap capabilities | get power cap | combined to be part of get power cap. |
|  set power cap | set power cap | |
|  get system parameters  | *limited* | system parameters will not be an API; there will likely be some system configuration, but we did not want to 'glue' ourselves into a specific implementation by making it part of the implementation where it is subject to `backwards compatibility` rules.
|  get node rules | *very limited* | there may be some node level rules, but we are skeptical that this information exists; we are relying on `YAGNI - You Ain't Going to Need It` and deferring this part of the work until we are proven wrong.  Any work we do now for sure would be wrong and would lock us into `backwards compatibility` rules. |
|  get group status | get power status | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  group off | transition xname | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  group on | transition xname | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  group reinit | transition xname | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  get node status | get power status | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  node off | transition xname | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  node on | transition xname | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  node reinit | transition xname | PCS does everything based on xname; groups, nids, partitions are not supported. This is covered in [the principle of identity](../Principles.md) |
|  get xname status | get power status | |
|  xname off  | transition xname | |
|  xname on | transition xname | |
|  xname reinit | transition xname | |
|  set p-state | -- | It was decided by the system architects that we could rely on the in-band mechanisms for this and that we did not need to expose an API for this; and further more that if we did, we would not do it in PCS. |
|  get p-state | -- | It was decided by the system architects that we could rely on the in-band mechanisms for this and that we did not need to expose an API for this; and further more that if we did, we would not do it in PCS. |
|  set c-state | -- | It was decided by the system architects that we could rely on the in-band mechanisms for this and that we did not need to expose an API for this; and further more that if we did, we would not do it in PCS. |
|  get c-state | -- | It was decided by the system architects that we could rely on the in-band mechanisms for this and that we did not need to expose an API for this; and further more that if we did, we would not do it in PCS. |
|  get nid map | -- | this is Hardware State Manager data, we won't repeat it as its not needed for PCS purposes. |
|  get node energy | -- | split out from PCS, All environmental telemetry is part of the Shasta Monitoring Framework. |
|  get node energy counter | -- | split out from PCS, All environmental telemetry is part of the Shasta Monitoring Framework. |
|  get node energy stats | -- | split out from PCS, All environmental telemetry is part of the Shasta Monitoring Framework. |
|  get system power | -- | split out from PCS, All environmental telemetry is part of the Shasta Monitoring Framework. |
|  get system power details | -- | split out from PCS, All environmental telemetry is part of the Shasta Monitoring Framework. |



### Deprecation Strategy (Approximate)

Our general 'be nice' deprecation strategy for all Cray Systems Managment APIs is to announce deprecation and then over the course of several releases give users a chance to acclimate to the changes. This is to avoid a nasty and unfortunate conflict of expectations where APIs are removed with no communication. 

The current (2021Q3 Plan) is to deprecate CAPMC over the next few releases with an official sundown expected as part of CSM v1.5. This means that any and all callers and clients will need to have adjusted to PCS no later than the release of CSM 1.5 because it will not contain CAPMC.  

The tactical strategy is to begin development on PCS as soon as possible and to release it as soon as it is ready.  As part of releasing PCS we plan on carving out CAPMC internals and making PCS calls under the hood. PCS will be the actual proxy and actor of power state, and CAPMC will be a temporary shell on top of it until is killed.

CAPMC has entered an enhancements freeze and will only get critical bug / security fixes. Any faults in the primary use cases that are not being ported to PCS (ie telemetry) will not be fixed. 

### Appendix 

#### Shasta Capmc Command Features

*This is for conversational reference*

| Shasta CAPMC Command | CAPMC Description |
| --- | --- |
| `node_on` | Controls powering on Nid[]; |
| `node_off` | Controls powering off Nid[]; |
| `node_reinit` | issues a `restart` or off/on sequence by Nid - aka reboot|
| `get_node_status` |  Returns the state manager status of the nodes.  |
| `get_xname_status` | Returns the state manager status of the xnames |
| `xname_on` | Controls powering on xname[]; It has the concept of a 'hierarchy'|
| `xname_off` | Controls powering off xname[]; It has the concept of a 'hierarchy'|
| `xname_reinit` | issues a `restart` or off/on sequence by xname - aka reboot|
| `group_on` |goes to HSM gets the xnames from the group; then powers on via redfish |
| `group_off` | goes to HSM gets the xnames from the group; then powers off via redfish |
| `group_reinit` | goes to HSM gets the xnames from the group; then powers off / on via redfish |
| `get_group_status` | goes to HSM gets the xnames from the group; then gets the POWER status via redfish |
| `partition_on` | goes to HSM gets the xnames from the partition; then powers on via redfish |
| `partition_off` |  goes to HSM gets the xnames from the partition; then powers off via redfish |
| `partition_reinit` | goes to HSM gets the xnames from the partition; then powers off / on via redfish |
| `get_partition_status` | goes to HSM gets the xnames from the partition; then gets the POWER status via redfish |
| `emergency_power_off` | EPOs the mountain components only |
| `get_power_cap_capabilities` | informs third-party software about installed hardware and its associated properties. Information returned includes the specific hardware types, NID membership, and power capping controls along with their allowable ranges. |
| `get_power_cap` |  can be queried for a set of nodes, gets the power cap; per node |
| `set_power_cap` | can be set for a collection of nodes AND GPUs,  Maximum amount of power the node can consume.  We set it via redfish and its a hardware limit (think a fuse). We just set it; it enforces itself. |
| `get_node_rules` | This information is used by the workload managers as well as site admins | hardware (and perhaps site-specific) rules or timing constraints that allow for efficient and effective management of idle node resources : e.g. time it takes to power things up and down|
| `get_system_parameters` |  Read-only parameters such as expected worst case system power consumption, static power overhead, or administratively defined values such as a system wide power limit, maximum power ramp rate, and target power band.  |
| `get_nid_map` | `get_nid_map` | NO | available in HSM; dont need to repeat it | mapped nids to cnames |
| `get_partition_map` | map of HSM partition |
| `get_node_energy` | returns node energy for an xname bound by time|
| `get_node_energy_stats` | returns node energy stats for an xname bound by time |
| `get_node_energy_counter` returns node energy counter for an xname bound by time|
| `get_system_power` | returns system power for the system bound by time |
| `get_system_power_details` | returns system power for a cabinet bound by time |
