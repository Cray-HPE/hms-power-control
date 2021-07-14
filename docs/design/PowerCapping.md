# Power-Capping
There are a few predominate definitions of power capping:

Power capping as a "capability" is the act of adjusting the amount of power a node or accelerator is allowed to consume.  This setting, applied to the BMC, is managed by the onboard management capabilities of the device and is by itself is not part of a larger control mechanism; e.g. that is to say there is no system wide power capping, power capping is done on a per device (node) basis. The power cap is a value within a range. i.e. 640w with a maximum of 900w and a minimum of 600w.

Power capping as a "HPC concept" is a strategy for getting the most computation ability out of your system as its power budget could be less than what the max load the hardware could draw could be. Workload managers and site administrators will use device power capping to apply larger 'power capping' strategy to sets of components to achieve site wide goals, e.g. stay within a power budget for monetary reasons or stay below a power usage to avoid over provisioning power resources (too much load).   

PCS exposes power capping as a capability, not as a strategy. PCS has created (extended the CAPMC API for power capping) which allows administrators and workload managers to implement their own power capping strategy. 

For the purpose of this document we will refer to power capping as a capability rather than a higher level system strategy. 

Power cap information is pretty simple, it has a range of allowed values (minimum, maximum), the current value that the device is set to, and an identifier for which part of the component the cap is for (accelerator 1 vs 2 vs node, etc). 

## Snapshots

The most common use case we identified was to retrieve the power cap on more than one components at a time.  Because this requires going to the redfish device and querying the hardware this can take time and is constrained by scaling and hardware reliability.  Therefore we created a snapshots endpoint for taking a point in time 'snapshot' of the current value, along with the capabilities (min, max) of the xname, and made it a non-blocking call that returns an ID.  The snapshot will self purge from the system after 24 hours (configureable at the service level).

## Patch

The second most common use case was setting a new power cap on more than one components at a time. Like 'snapshots' this requires going to the redfish device and setting the value, so can take time and is constrained by scaling and hardware reliability. The patch endpoint also returns an ID that can be queried and will return the current power cap information as well (to give context) to the callers request. Like snapshots, the record of the patch will self purge from the system after 24 hours.
