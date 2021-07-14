# Power-Status
 *Can you ask a computer if it is off?*

A foundational topic in hardware management is determining the power state of a device. If we were in a data center we could go look at the device, we could probably see lights blinking, maybe even hear or feel the mechanical vibration of the disks or fans, the heat of the exchange, or we could even use a current sensor and measure the current draw.

If a device has an `xname` and is tracked in hardware state manager, we want to be able to report power state for that device.  This does not mean that an actor can manage the power state of that component; merely that we can be informed of its state.

The proposed state model we will support is:

* `on`
* `off`
* `undefined`

Universally, the most basic state is `on`. The `on` state can be directly communicated, or can be inferred, e.g. if a device returns a `ping` it must have enough of an operational software stack to do so, and is therefore `on`.  

Power state information can be programmatically derived from several sources:

1. Information 'PUSHED' from the device
   1. heartbeats
   2. events
2. Information 'POLLED' from the device
   1. pings
   2. other network requests
3. Information from a separate device
   1. BMC:
      1. Manager status (Redfish, IPMI, etc)
      2. Telemetry & Sensor data

While `on` can be inferred, `off` cannot. Failure to respond to `ping` or `ssh` does not mean that the device is in fact actually powered `off`, there could be any number of reasons why a device does not respond (see the OSI model). Obviously and paradoxically a device cannot indicate if it is off, only if it is on. 

We must leverage the information from 'other' devices to help determine power state. The only authoritative source of knowing if a device is `off` is its BMC, or the PDU (power controller), or another controller above the device in the power hierarchy.  The BMC has the appropriate hardware to know if a node is off, and a PDU (a switchable outlet) can physically remove power from a device.  

In the case of nodes, this 'other device' is its BMC. In the case of a BMC or other controller, this is likely a controller or PDU at a higher level in the power hierarchy; e.g. the 'slot' controls the power to the nodeBMC. Most controllers, at least all we are familiar with, do not have 'power on' or 'power off' functionality. If the circuit is energized they will turn on, short a hardware/firmware fault. 

The distinction regarding `off` and 'functionally' `off` is important because it is possible that a device is operational, doing work, but that the inability or failure to recognize it as `on` may lead to an actor taking a disruptive action, e.g. restarting it. Therefore we need an additional state `undefined`, to represent that we do not really know the state of the device; it could be `on` and not responding, or could in fact be 'functionally' `off`.

`on` will be determined via:

* polling the device status via Redfish
* pinging the device
* listening for events or heartbeats

`off` will be determined via:

* the BMC (or power hierarchy controller) reporting the device is off

`undefined` will be used when:

* the device fails to respond and we cannot confirm a positive `on` or a positive `off`, after some period of time of non-response we may declare the device `off` (TBD).

The advantage of this disparate data flow is that we can rely on many data sources to understand the power state of a device. PCS can directly poll the BMC, but can also use information provided by the device itself to know its state.

## Abstracted States

Redfish has limited power states that are not universally supported.  They typical list of power states are:
   1. `on`
   2. `powering on`
   3. `off`
   4. `powering off`

`powering on` implies that the device is `on` but not yet ready to receive commands.
`powering off` implies that the device is `off` but will soon stop responding to commands. 

We will abstract the states to:

   1. `on`
   2. `off`
   3. `undefined` - the power state cannot be determined, this should hopefully be rare

## Management State

In addition to `power-state` we care about the management state of the device, which answers 'can the device be currently controlled via its known control mechanisms?'.  `management-state` is either `available` or `unavailable`. This further distinction is important because it represents that the 'manager' (often the BMC) is `available` to be interacted with or is `unavailable`. 

A use case for this information is when a BMC (baseboard management controller) is overloaded and is not responding to requests in a timely manner, but the node is heart beating (powered on), running a compute workload. An admin in this case that wants to control the node power state needs to know the context, the node is alive, but the manager is not currently responding.  This added context might lead the admin to adopt a wait-and-see approach, or determine that they don't need to take an intervention immediately. 

## Supported Power Transitions

Part of the PCS model is that components have varying levels of support of power transitions.  For example, nodes can typically be hard/soft restart, turned on or off, init, forced off. BMCs, however usually can only be restarted, their power topology doesn't allow them to be transitioned `on` or `off` because if they are plugged in, they are on.  The power status API will show a list of valid power transitions for the xname given its component type.