## Developer Archeology
*How we got to here...*

This file is a draft listing of our comparison of XC CAPMC vs Shasta CAPMC vs PCS. It is maintained for historical context

### Notes -> Might be useful; but don't going digging too hard here gentle reader! 

 * *The final authority on what our software does is the code!*
 * *The final authority of what the software should do is the swagger and design doc!*
 * *The reasonings and how we got to where we are is in the notes*

## API Map - For Developer conversation


| Equivalent XC | Shasta CAPMC | Shasta PCS Plan | Justification | CAPMC Description |
| --- | --- | --- | --- | --- | --- |
| `node_on` | `node_on` | - | Admins and WLMs want to power things on by Nid, however from the API perspective we must be xname centric| Controls powering on Nid[]; |
| `node_off` | `node_off` | `transitions` | Admins and WLMs want to power things on by Nid, however from the API perspective we must be xname centric| Controls powering off Nid[]; |
| `node_reinit` | `node_reinit` | `transitions` | its valuable to be able to say restart vs 'off on' | issues a `restart` or off/on sequence by Nid - aka reboot|
| `get_node_status` | `get_node_status` | `power-status` | The WLM will have to use Xnames as nids are not partition unique or multitenant unique.| Returns the state manager status of the nodes.  |
| - | `get_xname_status` | `power-status` | | Returns the state manager status of the xnames |
| - | `xname_on` | `transitions` | BOA wants to power things on by xname| Controls powering on xname[]; It has the concept of a 'hierarchy'|
| - | `xname_off` | `transitions` | BOA wants to power things off by xname| Controls powering off xname[]; It has the concept of a 'hierarchy'|
| - | `xname_reinit` | `transitions` | its valuable to be able to say restart vs 'off on' | issues a `restart` or off/on sequence by xname - aka reboot|
| - | `group_on` | YES | as a concept its valuable to be able to control a 'group' | goes to HSM gets the xnames from the group; then powers on via redfish |
| - | `group_off` | YES | as a concept its valuable to be able to control a 'group' | goes to HSM gets the xnames from the group; then powers off via redfish |
| - | `group_reinit` | YES | as a concept its valuable to be able to control a 'group' | goes to HSM gets the xnames from the group; then powers off / on via redfish |
| - | `get_group_status` |  YES | as a concept its valuable to be able to control a 'group' | goes to HSM gets the xnames from the group; then gets the POWER status via redfish |
| - | `partition_on` | YES | as a concept its valuable to be able to control a 'partition' | goes to HSM gets the xnames from the partition; then powers on via redfish |
| - | `partition_off` | YES | as a concept its valuable to be able to control a 'partition' | goes to HSM gets the xnames from the partition; then powers off via redfish |
| - | `partition_reinit` | YES | as a concept its valuable to be able to control a 'partition' | goes to HSM gets the xnames from the partition; then powers off / on via redfish |
| - | `get_partition_status` |  YES | as a concept its valuable to be able to control a 'partition' | goes to HSM gets the xnames from the partition; then gets the POWER status via redfish |
| - | `emergency_power_off` | YES |  | EPOs the mountain components only |
| `get_power_cap_capabilities` | `get_power_cap_capabilities` | YES | we generate the information from Redfish; this API is used by the admins (Maybe by the WLM, but...) We actually get this data from HSM... so we might defer to HSM| today: informs third-party software about installed hardware and its associated properties. Information returned includes the specific hardware types, NID membership, and power capping controls along with their allowable ranges. |
| `get_power_cap` | `get_power_cap` | YES | can be queried for a set of nodes | gets the power cap; per node |
| `set_power_cap` | `set_power_cap` | YES | can be set for a collection of nodes AND GPUs| Maximum amount of power the node can consume.  We set it via redfish and its a hardware limit (think a fuse). We just set it; it enforces itself. |
| `get_freq_capabilities` | - | `get_freq_capabilities` (p-state) | this is from the running OS; not Redfish. The interface has not been defined. | |
| `get_freq_limits` | - | `get_freq_limits` (p-state) | this is from the running OS; not Redfish. The interface has not been defined. | |
| `set_freq_limits` | - | `set_freq_limits` (p-state) | this is from the running OS; not Redfish. The interface has not been defined. | |
| `get_sleep_state_limite_capabilities` | - | `get_sleep_state_limite_capabilities` (if needed) | this is from the running OS; not Redfish. The interface has not been defined. | |
| `set_sleep_state_limit` | - | `set_sleep_state_limit`  | this is from the running OS; not Redfish. The interface has not been defined. | |
| `get_sleep_state_limit` | - | `get_sleep_state_limit`  | this is from the running OS; not Redfish. The interface has not been defined. | |
| Deprecated | Deprecated | Deprecated | Deprecated | Deprecated |
| `get_node_rules` | `get_node_rules` | NO | This information is used by the workload managers as well as site admins | hardware (and perhaps site-specific) rules or timing constraints that allow for efficient and effective management of idle node resources : e.g. time it takes to power things up and down|
| `get_system_parameters` | `get_system_parameters` | NO | This data MAY be useful for a few PCS functions: power scaling, static power overhead (might be needed for power scaling). ; unsure what other data is really needed for the service | Read-only parameters such as expected worst case system power consumption, static power overhead, or administratively defined values such as a system wide power limit, maximum power ramp rate, and target power band.  We will not expose this as an API, it may be a configmap.  This could later become an API if needed. |
| `get_nid_map` | `get_nid_map` | NO | available in HSM; dont need to repeat it | mapped nids to cnames |
| `get_partition_map` | - | NO | This is HSM information| |
| get_mcdram_capabilities (Xeon Phi) | - | - | mcdram is not a thing; and if it was; its not power thing so not a good fit for PCS | |
| get_mcdram_cfg (Xeon Phi) | - | - |mcdram is not a thing; and if it was; its not power thing so not a good fit for PCS | |
| set_mcdram_cfg (Xeon Phi) | - | - |mcdram is not a thing; and if it was; its not power thing so not a good fit for PCS | |
| clr_mcdram_cfg (Xeon Phi) | - | - |mcdram is not a thing; and if it was; its not power thing so not a good fit for PCS | |
| get_numa_capabilities (Xeon Phi) | - | - | The numa config was part of KNL which isnt something we have in our POR and isnt a good fir for PCS | |
| get_numa_cfg (Xeon Phi) | - | - | The numa config was part of KNL which isnt something we have in our POR and isnt a good fit for PCS | |
| set_numa_cfg (Xeon Phi) | - | - | The numa config was part of KNL which isnt something we have in our POR and isnt a good fit for PCS | |
| clr_numa_cfg (Xeon Phi) | - | - | The numa config was part of KNL which isnt something we have in our POR and isnt a good fit for PCS | |
| get_ssd_enable (XC Only) | - | - | SSD data is not power data; so we wont put it in PCS | |
| set_ssd_enable (XC Only) | - | - | SSD data is not power data; so we wont put it in PCS | |
| clr_ssd_enable (XC Only) | - | - | SSD data is not power data; so we wont put it in PCS | |
| get_ssds (XC Only) | - | - | SSD data is not power data; so we wont put it in PCS | |
| get_ssd_diags (XC Only) | - | - | SSD data is not power data; so we wont put it in PCS | |
| get_power_bias | - | - | Trimming is not a requirement for Shasta | |
| set_power_bias | - | - | Trimming is not a requirement for Shasta | |
| clr_power_bias | - | - | Trimming is not a requirement for Shasta | |
| set_power_bias_data | - | - | Trimming is not a requirement for Shasta | |
| compute_power_bias | - | - | Trimming is not a requirement for Shasta | |
| get_node_energy | get_node_energy | NO | Telemetry has been removed from our scope | |
| get_node_energy_stats | get_node_energy_stats | NO | Telemetry has been removed from our scope | |
| get_node_energy_counter | get_node_energy_counter | NO | Telemetry has been removed from our scope | |
| get_system_power | get_system_power | NO | Telemetry has been removed from our scope | |
| get_system_power_details | get_system_power_details | NO | Telemetry has been removed from our scope | |