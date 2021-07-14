Implementation
We need to be able to cache creds in memory in PCS; that way we can survive Vault not always being there : see CASMHMS-3978


API
We should have consistent response structures in the API; https://connect.us.cray.com/jira/browse/CASMHMS-3968
Add a version to the API
Need an service Status/ Health/ API
Service level configurability: polling intervals, logging level, etc. (See HBTD)
https://connect.us.cray.com/confluence/pages/viewpage.action?spaceKey=SMA&title=REST+API+Best+Practices
Write errors in RFC7807


DESIGN
needs a BMC agnostic way of indicating power action no-op; https://connect.us.cray.com/jira/browse/CASMHMS-3679
need a way to expose BMC level errors to give context: https://connect.us.cray.com/jira/browse/CASMHMS-3442
power actions need to retry in event of failure: https://connect.us.cray.com/jira/browse/CASMHMS-1927
Implement WaitForPowerState function -> eg the core logic for power transitions: https://connect.us.cray.com/jira/browse/CASMHMS-1403 
PCS needs to understand the power hierarchy as part of its design: https://connect.us.cray.com/jira/browse/CASMHMS-1390 -> Reconcile xnames we cannot directly control from xnames we can control



## Future Features and updates

* Backend performance improvements
* Moving to a truly RESTful interface (v2)
* Power control
  * Emergency Power Off at the iPDU levels
  * Power control of Mountain CDUs (won't/cant do)
  * Power control policies
  * Power control of Motivair door fans
  * Power control of in-rack River CDUs
* Power capping and related for Mountain
  * Group level and system level power capping (if needed) > Needed?
  * RAPL (Running Average Power Limiting) (if possible) > Needed?
* Powering off idle nodes (most likely a WLM function) (This is a use case)
* Rebooting nodes (most likely a CMS or WLM function) (This ia a use case)

## Limitations

* No Redfish interface to control Mountain CDUs
* CMM and CEC cannot be powered off. They are always ON when Mountain cabinets are plugged in and breakers are ON
* Can only talk to components that exist in HSM
* We only do power control on things we can directly power off, e.g. if we can remove power from the device (by powering off the slot) we don't include that, it must be something we can talk to (NOT SURE IF THIS IS GOOD)



PCS can only power control:

 * "CabinetPDUPowerConnector", 
 * "CabinetPDUOutlet", 
 * "Chassis", 
 * "RouterModule", 
 * "HSNBoard", 
 * "ComputeModule", 
 * "Node"


List of components in HSM:  (`cray hsm state components list | jq '.[]|.[]|.Type' | sort | uniq -c`)
 * "CabinetPDU"
 * "CabinetPDUController"
 * "CabinetPDUPowerConnector"
 * "Chassis"
 * "ChassisBMC"
 * "ComputeModule"
 * "HSNBoard"
 * "Node"
 * "NodeBMC"
 * "NodeEnclosure"
 * "RouterBMC"
 * "RouterModule"




## TODOS

Come up with a concise list of already identified user focused scenarios : distinguish between CLI and API


Scenario: 

Person A requests to turn off
Person B has already taken the reservation and now want to power on the xnames.

We cannot wait for A to be fulfilled for B to start; because it never will be.

There is no queueing if you cannot acquire the reservation; therefore there is no queueing.

So if person XXX requests and it cannot get the reservation then the transition must fail (either in part or at the task level). So if 99/100 things got the reservation and ONE didn't; that ONE is DEAD for the purpose of the transition.  We wont do an all or nothing; at least for now.   The all or nothing *could be done up to the point we actually start issuing commands to BMCs


TODO: have a conversation about dynamic configuration (eg being able to update logging level on the fly)