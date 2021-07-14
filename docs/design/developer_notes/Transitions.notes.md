## Developer Archeology
*How we got to here...*

Like all things in the PCS space, we got here after much consideration and discussion.  We reviewed it many times and took an iterative approach.  What we settled on represents a balance between the foundational elements of the past and a realization that we needed to think about this problem in a new way.  Our main takeaway is that many more components can be 'power transitioned', to some degree or another.  The leading principle is simplicity, tell us what to transition, and we will do it!

### Notes -> Might be useful; but dont going digging too hard here gentle reader! 

 * *The final authority on what our software does is the code!*
 * *The final authority of what the software should do is the swagger and design doc!*
 * *The reasonings and how we got to where we are is in the notes*


#### Transitions Notes:

Need a way to specify a hierarchy? the most common way is to go 'down' the hierarchy on a power off: eg; turn THIS and all below it off.

A list of things we can actually power off: 

* "CabinetPDUPowerConnector" === x3000m0p0v0 (COTS -> River. TORs, TORethernet, RiverCDUs
* "Chassis" === x1000c0 
* "RouterModule" === x1000c0r0 (This is slot power (which is how the rosetta and BMC get power) 
* "HSNBoard" === x1000c0r0e0 (this is the rosetta, not its BMC)
* "ComputeModule" === x1000c0s0 (this is the slot)
* "Node" === x1000c0s0b0n0

I think we need to come up with the abstraction of things that can be powered down: TODO?

* slots (takes BMCs and Nodes down with it)
* rectifier?
* Chassis (takes down the slots)
* nodes

The abort makes sense in context of a ramp rated power on action; if there is a delay of 10 seconds per group we dont want to have them time out for XXX seconds waiting for it to complete.  but usually it should be so fast it doesnt matter

we are going to have a configurable 'delete' for the transitions; what does this mean? I know that ADN wrote this... but I think twas based on what MJ said...


#### Implementation Considerations 
 * consider adding a user agent header into the POST for the transitions as a way to gain observability; this may not be needed based on how ISTIO routing is done.
 * We need to talk HMS team about putting SLOT components into state manager
 * What do we do about reservations? how long are transitions valid for?  b/c the state kinda goes stale... e.g.  the transition has completed; but the thing may have been rebooted now. I believe our conclusion was that we were going to all for deputy keys to be passed in to us, and the reason why we call it 'transitions' and not 'state'. we will 'stop looking' after we get one positive confirmation; Its impossible to guarantee the hardware state; because its not always 100% reliable. Too much jitter in the hardware.
 * we arent putting a hierarchy flag in the API; but rather hierarchies can be built in the CLI.  the API accepts xnames.  They ONLY hierarchy stuff PCS should intervene with is when hardware damage is possible. So we need to be power relationship aware: 'whats plugged into what'; but we wont by default assume a set of transitions 'inherently' - people have to be explicit. 
 * an important use case to remember is it take a `force-off` to chassisBMCs to clear an EPO.
 * Administrators do not want management stuff to go down unless they say so intentionally! This is where locking/reservation API is useful... Not sure what role the CLI plays in this... maybe a 'are you sure' type of thing?