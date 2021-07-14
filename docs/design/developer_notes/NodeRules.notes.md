## Developer Archeology
*How we got to here...*

Like all things in the PCS space, we got here after much consideration and discussion.  We reviewed it many times and took an iterative approach.  After much discussion we decided that the configuration APIs (node rules, system parameters) were not a good fit for the first version of PCS for reasons [outlined elsewhere ](../CAPMC_vs_PCS/NodeRules-SystemParameters.md).

### Notes -> Might be useful; but don't going digging too hard here gentle reader! 

 * *The final authority on what our software does is the code!*
 * *The final authority of what the software should do is the swagger and design doc!*
 * *The reasonings and how we got to where we are is in the notes*

What is the relationship between `node rules`, `system parameters` and `node capabilities (power, sleep, frequency)`

* Capabilities are 'discoverable' e.g. we can go query something (redfish, or the OS).
* The node rules and systems parameters are 'configs' that ship with CAPMC. They reflect some of the limits of the system as a whole; or the nodes themselves. 
* The node rules are REALLY just guidelines; they have no teeth.  needs to broken up by hardware type; the system based stuff is general across everything.    HOWEVER maybe these nodes need to actually be part of our control algorithm. e.g. you can only ask for 10 things on at a time; so if you ask for 12; we say no.
```
jolt1-ncn-w001:~ # cray capmc get_node_rules list --format json
{
  "max_off_req_count": -1,
  "max_on_req_count": -1,
  "e": 0,
  "latency_node_reinit": 180,
  "err_msg": "",
  "latency_node_off": 60,
  "min_off_time": -1,
  "latency_node_on": 120,
  "max_off_time": -1,
  "max_reinit_req_count": -1
}
jolt1-ncn-w001:~ # cray capmc get_system_parameters list --format json
{
  "power_threshold": 0,
  "e": 0,
  "ramp_limit": 2000000,
  "static_power": 0,
  "err_msg": "",
  "ramp_limited": false,
  "power_band_min": 0,
  "power_cap_target": 0,
  "power_band_max": 0
}
```

## Node rules


This is really device type rules; 

There is a Class: mountain, river.
There is a hsm type (node, etc)

There is some mfg information -> comes from redfish; the CRAY stuff is changing
there is model information -> comes from redfish; the CRAY stuff is changing 


These 'rules' are heuristics for the hardware; and then CAPMC would use that information to know 'how' (really delays, latency; on off times. re-init times.  (including OS).  Max number of nodes of this type that can be powered on at the same time.   MIN or MAX off time -> time in S in which it might be in the off state.   However CAPMC didn't use this; and this is antithetical to our model!


Its unclear if this is used; or how it is used. We need to ask the WLM folks


## System Params

power cap target : admin defined upper limit on system power

Static system power overhead : unreported -> DOCS only; Not putting it in PCS

Ramp Limited : is this system power ramp limited. admin define rate of change 


static system power + measurable power MUST BE <= power cap target. 

What happens if we exceed the values? do **we** turn peoples nodes off?! *rhetorical*; NO. Do we prevent all ON events?  NO; so we really cannot do anything about it; therefore we should not care about it;  Its important; its just not in our operating box. 

This *important system configuration information belongs somewhere outside of PCS; maybe SLS?
WHere is the data going to get used?
if its used by a service; then put it in a service; else, write it down in a notebook.

We need to find out if the WLMs read this information; they are the only ones who really could do something about it.

---

power band min, power band max -> admin defined min/max power consumption. allows an admin to convey an external constrain to a WLM;  DO the WLM's actually use this? 

err_msg (string) – Human readable error string indicating failure reason

* `power_cap_target` (int) – Administratively defined upper limit on system power
* `power_threshold` (int) – System power level, which if crossed, will result in Cray
management software emitting over power budget warnings
* `static_power` (int) – Additional static system wide power overhead which is unreported,
specified in watts
* `ramp_limited` (bool) – true if out-of-band HSS power ramp rate limiting features are
enabled
* `ramp_limit` (int) – Administratively defined maximum rate of change (increasing or
decreasing) in system wide power consumption, specified in watts per minute
* `power_band_min` (int) – Administratively defined minimum allowable system power
consumption, specified in watts
* `power_band_max` (int) – Administratively defined maximum allowable system power
consumption, specified in watts
