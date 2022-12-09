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
 
### Implementation notes

#### Order of Operations for Transitions Goroutine

1. Start keep alive goroutine to update LastActive timestamp to prevent the reaper goroutine from restarting our transition.
2. Create tasks for each xname and check for previously created tasks associated with our transition to form a task list. This removes duplicates and creates failed/unsupported tasks for invalid xnames and unsupported component types. 
3. Update the transition record in ETCD to in-progress (checks for abort signals before storing)
4. Get the power state records from ETCD for each component and any child components. This way we already have any HSNBoard information if we need to add it.
5. Get HSM data for each component and any child components. Same reason as #4.
6. Create tasks for any existing HSNBoard components if the requested transition will cause the parent slot to be powered off and they were not requested.
7. Update the transition record in ETCD (checks for abort signals before storing).
8. Sort the components into groups by operation type ("on", "gracefulshutdown", etc.) and by HMSType for power sequencing.
9. Reserve components in HSM failing any we can't reserve.
10. Start TRS task operations looping for each tier in the power sequence.
    1. Check for abort signal.
    2. Check component reservations in HSM failing any we nolonger hold.
    3. Form the TRS task list, get redfish credentials, and generate payloads for the POST operations.
    4. Launch the TRS tasks for POSTing the power commands.
    5. Upon response, update the task record in ETCD with Status=in-progress and State=Waiting if successful. Fail if not successful.
    6. After all POST responses have been received and processed, start waitFor loop to confirm the transition.
        1. Check for abort signal.
        2. Sleep for 15 seconds to allow time before and between querying the hardware for power state.
        3. Form the TRS task list and get redfish credentials for the GET operations.
        4. Launch the TRS tasks for GETing the hardware power status.
        5. Upon response:
            * If the component is in the desired ending power state and this is the only operation being done on this component, update the task record in ETCD with Status=Success. Remove from waiting list.
            * If the component is in the desired ending power state and this is not the last operation being done on this component, update the task record in ETCD with State=Confirmed. Remove from waiting list.
            * If an HTTP failure occured, fail the component. Remove from waiting list.
            * Otherwise, do nothing.
        6. After all GET responses have been received and processed, check if our waiting list length and time elapsed for waiting:
            * If the wait list is empty, start the next tier in the power sequence (go to 10).
            * If the time elapsed is greater than our TaskDeadline (default 5 mins) and the operation allows for use of force, add the leftover components in this tier to the force[off/on/restart] sequence tier if not already. Start the next tier in the power sequence (go to 10.6).
            * If the time elapsed is greater than our TaskDeadline (default 5 mins) and the tier is force[off/on/restart] or the operation doesn't allow for the use of force (soft-off), fail the leftover components. Start the next tier in the power sequence (go to 10.6).
            * Otherwise, start another waitFor loop (go to 10.6)
    7. After the last power sequence tier has been processed, break from the loop.
11. Update the transition record in ETCD to complete.
12. Release component reservations in HSM.

#### About The Power Sequence

The order of which groups of components have power transitions applied is controlled by [PowerSequenceFull](https://github.com/Cray-HPE/hms-power-control/blob/develop/internal/domain/transitions.go#L60-L104) array.
Each element in the array is a tier that specifies a power action to be applied to all components of a list of HMSTypes in a single TRS launch.
The array is executed in order thus specifying, for example, Nodes and HSNBoard components will first get a `gracefulshutdown/off` then Nodes and HSNBoard components will get a `forceoff`.
The power sequence being used is:

1. GracefulShutdown/Off Nodes and HSNBoards
2. ForceOff Nodes and HSNBoards
3. GracefulShutdown/Off RouterModules and ComputeModules
4. ForceOff RouterModules and ComputeModules
5. GracefulShutdown/Off Chassis
6. ForceOff Chassis
7. GracefulShutdown/Off CabinetPDUPowerConnectors
8. ForceOff CabinetPDUPowerConnectors
9. GracefulRestart all types (Only used if supported and parent components are not going to drop power)
10. On CabinetPDUPowerConnectors
11. On Chassis
12. On RouterModules and ComputeModules
13. On Nodes and HSNBoards

The `PowerSequenceFull` array is used in conjunction with a golang map, seqMap, that groups components by power action then by HMSType to determine which components, if any, are to be operated on in each tier.
Tiers with no components to operate on are simply skipped.
Any one component can exist multiple times within the seqMap but only once within a single tier. For example, PCS HardRestart will cause all components to be placed in both `seqMap["gracefulshutdown"][<HMSType>]` and `seqMap["on"][<HMSType>]`.
They are initially placed (before starting TRS operations) in groups using the following logic:

 * For PCS Off/SoftOff operations, components are placed in `seqMap["gracefulshutdown"][<HMSType>]`.
 * For PCS ForceOff operations, components are placed in `seqMap["forceoff"][<HMSType>]`.
 * For PCS On operations, components are placed in `seqMap["on"][<HMSType>]`.
 * For PCS SoftReset operations, components that support it are placed in `seqMap["gracefulrestart"][<HMSType>]`.
 * For PCS HardReset and SoftReset (when unsupported by the component) operations, components are placed in `seqMap["gracefulshutdown"][<HMSType>]` and `seqMap["on"][<HMSType>]`.
 * For PCS Init operations, components that are `On` are placed in both `seqMap["gracefulshutdown"][<HMSType>]` and `seqMap["on"][<HMSType>]`.
 * For PCS Init operations, components that are `Off` are placed in only `seqMap["gracefulshutdown"][<HMSType>]`.

Components that timeout during `GracefulShutdown` operations get added to `seqMap["forceoff"][<HMSType>]` (except for PCS SoftOff operations).

#### The Reaper Goroutine

The reaper goroutine gets started when PCS starts an runs periodically deleting expired records and picking up abandoned transitions. It follows the following pattern:

1. Get all transition records
2. Delete expired records that have the Aborted or Completed status.
3. Signal abort for expired records that are in-progress.
4. Pick up abandoned transitions.

Expired transitions are those who's `AutomaticExpirationTime` has passed. This is generally 24 hours after creation.
A transition is considered abandoned when the transition's `LastActiveTime` hasn't been updated after 3 times the `KeepAliveInterval` (10 seconds).
When picking up abandoned transitions, the reaper goroutine updates the transition's `LastActiveTime` and uses an ETCD TAS operation to store it. This prevent multiple instances from picking up the same abandoned transition.

#### Keep Alive

Each running transition starts its own keepAlive() goroutine. This goroutine periodically updates the transition's `LastActiveTime` in ETCD. R/M/TAS operations are used to prevent stepping on updates from other instance/goroutines.
The keepAlive goroutine spins every `TransitionKeepAliveInterval` (10 seconds) so the reaper goroutine doesn't consider the transition to be abandoned.
The `LastActiveTime` must be updated once every 3*`TransitionKeepAliveInterval` (30 seconds) to prevent it from being considered abandoned by the reaper.
The keepAlive goroutine will stop itself if the transition record gets deleted or its status changes to Aborted or Complete. Otherwise, it can be killed using the cancel channel.

#### Aborting Transitions

Transitions are aborted with REST API calls to `DELETE /transitions/<ID>`. This call results in PCS updating the transition's status in ETCD to `abort-signaled`.
The goroutine that is running the transition uses TAS operations when updating the ETCD record. The TAS operation will fail if another PCS instance or goroutine has updated the transition record.
Upon TAS failure, the transition goroutine will try R/M/TAS checking for `abort-signaled` to see if it needs to clean up and exit.
The transition goroutine also checks ETCD specifically for the abort signal in specific spots during operation because it doesn't always need to update the transition record.
If an abort has been signaled, the transition goroutine will:
 * Mark any incomplete task as failed.
 * Update the transition status to `aborted` in ETCD.
 * Release any reservations.

#### Starting Abandoned Transitions

The reaper goroutine detects and picks up abandoned transitions. The PCS instance that picks up the abandoned transition first will start a new transitions goroutine for it.
When starting, the transition goroutine always checks the status of the transition and if there are any pre-existing tasks in ETCD for it.
Pre-existing task records track their progress using the `task.Operation` and `task.State` fields.
The `task.Operation` field differs from `transition.Operation` in that `transition.Operation` indicates the operation specified by the user and `task.Operation` indicates the power operation (On, off, etc.) being done on the component associated with the task.
For example, if the `transition.Operation` is `Init`, the `task.Operation` could be `Off`, `ForceOff`, or `On` depending on how far it has progressed in the transition.
The `task.State` field tracks what part of the TRS operation the task was in.

 * GatherData - Before Any TRS operation has started.
 * Sending - TRS operation was launched to POST the power command. Response has not been received.
 * Waiting - Response from the POST operation was received. Waiting to confirm power state change.
 * Confirmed - Confirmed expected power state change.

Together `task.State` and `task.Operation` indicate the progress made on an individual component in a PCS transition operation.
For example, for PCS Init, the task Operation/State for each component specified will progress similar to the following:
```
init/GatherData -> off/Sending -> off/Waiting -> off/Confirmed -> on/Sending -> on/Waiting -> on/Confirmed
```

The transitions goroutine uses these fields to place them in the correct power sequencing groups when starting an abandoned transition using the following logic:

 * If the `task.State` is `Sending` and the component doesn't seem to have change power state, place it in the sequence as normal to retry the POST.
 * If the `task.State` is `Waiting` and the component doesn't seem to have change power state, place it in the sequence as normal the POST will get skipped for this component due to `task.State`.
 * If the `task.State` is `Confirmed` and the component doesn't seem to have change power state, fail the task because something went wrong.
 * If the `task.State` is `Sending`/`Waiting`/`Confirmed`, the component has changed power state, and this is the last operation for the component, mark the task Status as succeeded.
 * If the `task.State` is `Sending`/`Waiting`/`Confirmed`, the component has changed power state, and this is not the last operation for the component, mark the task State as Confirmed and place only in the next operation in the power sequence.

