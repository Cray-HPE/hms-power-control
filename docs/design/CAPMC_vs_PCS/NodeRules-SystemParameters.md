# Elimination of Node Rules and System Parameters API

CAPMC had two similar, but slightly different endpoints for a closely related topic:

## Node Rules

```
node_rules informs third party software about hardware (and perhaps site-specific) rules or timing constraints that allow for efficient and effective management of idle node resources. The data returned informs the caller of how long node_on and node_off operations are expected to take, the minimum amount of time nodes should be left off to save energy, and limits on the number of nodes that should be turned on or off at once. Default rules are supplied where appropriate.
Other values such as the maximum node counts for node_on or node_off and the maximum amount of time a node should remain off after a power down are left unset. The values are not strictly enforced by Cray system management software. They are meant to provide guidelines for authorized callers in their use of the CAPMC service.
```


## System Parameters

```
Read-only parameters such as expected worst case system power consumption, static power overhead, or adminis- tratively defined values such as a system wide power limit, maximum power ramp rate, and target power band may be returned via the get_system_parameters call. Returned values are used to convey intent between the system administrator and external agents with respect to target power limits and other operational parameters. The returned parameters are strictly informational.
```

## PCS will not have these endpoints
PCS will not expose node rules or system parameters as an API for a few reasons. 

  1. CAPMC didn't actually need the data, it was a reference guide for callers
  2. PCS doesn't need the overwhelming majority of the data and neither will callers because of how it functions.  
     * PCS is a non-blocking API, CAPMC is mostly a blocking API; meaning that a call to CAPMC is likely to result in a long to indefinite wait while the system resolves the operation.  
     * PCS tokenizes the request and allows the caller to get status later.
  3. we are going to avoid the `chicken-and-the-egg` of how we get this data; because there is no one to source or supply it
  4. if we do need the data we will supply it in a config map and avoid the `backwards compatibility trap` that we would inevitably fall into by making it part of the API standard. If it develops usefulness we can always add it into the API later (`Just enough architecture`). 