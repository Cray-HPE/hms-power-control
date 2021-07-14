## Developer Archeology
*How we got to here...*

Like all things in the PCS space, we got here after much consideration and discussion.  We reviewed it many times and took an iterative approach.  What we settled on represents a balance between the foundational elements of the past and a realization that we needed to think about this problem in a new way.  Our main takeaway is that 'almost anything' can have power status. Not everything can be controlled, but many components have a power status; sometimes this power status is implicit or explicit, and sometimes the power status isn't really useful because there are no controls to operate on it (consider a light that is always on if the main breaker is engaged).  The leading principle is that we wanted to expose greater context to the caller so they could understand the status of the hardware and what was happening on it clearer.  

### Notes -> Might be useful; but don't going digging too hard here gentle reader! 

 * *The final authority on what our software does is the code!*
 * *The final authority of what the software should do is the swagger and design doc!*
 * *The reasonings and how we got to where we are is in the notes*

ADN  10:19
maybe this needs a call; but id like to revisit the ‘things’ we will show status for… It makes sense that you cannot ask a BMC to turn off; as once they are plugged in they are on, but I'm thinking if it has an xname shouldn't we really be able to ask it its power state?

MJ  10:20
Then people will ask why we can't turn it off
They already have, it confuses people.

ADN  10:20
when can say its not supported
I think its confusing the way it is now
It would be a valid question to ask if my BMC is on?

MJ  10:20
If the slot is on the BMC is on

ADN  10:21
right… but why hide that

MJ  10:21
If the outlet is On the BMC is On
Because it confuses people.

ADN  10:21
do we have xnames in HSM for sheet metal only?
eg will there be a ‘state components’ for a river chassis?

MJ  10:21
Technically in mountain the Chassis is the ChassisEnclosure
But we have named it as a Chassis to reduce confusion.
Right now in River the only components we control power on are Outlets and Nodes
there are no valid Chassis xnames in HSM for River

ADN  10:24
im going to propose that if its a valid xname in state components that we allow people to request a power transition and hardware status of it.   If we cannot actually power control it; it would come back as ‘unsupported’.

MJ  10:24
I am going to say no. It confuses people. It has already and it will again.

ADN  10:25
i think its confusing either way… I think the lack of parity is more confusing IMO -> but Ill draw it up as a proposal and we can dialogue on it in person

MJ  10:25
The xname is a hierarchy. People think they can power off the node x0c0s0b0n0, then the BMC x0c0s0b0, then the slot x0c0s0, then the chassis x0c0

ADN  10:26
does the BMC report power state? or is that implied by the fact you can redfish talk to it?

MJ  10:27
It is implied
It does have a status field
ncn-m001:~ # curl -sk -u root:$PASSWD https://x3000c0s22b0/redfish/v1/Managers/1 | jq '.Status'
{
  "Health": "OK",
  "State": "Enabled"
}
(edited)

ADN  10:28
can I ask a hair splitting question?
is it possible for slot power to be on, and the BMC to not be on? b/c the BMC is either A) rebooting or B) has a bad power module?

MJ  10:29
We are unable to determine if the BMC is powered On or if we have a networking issue.

ADN  10:30
ok; thats a fair point… is there anything in a BMC that says if its going to reboot?
I know for firmware updates theres some stuff; kinda

MJ  10:30
If the BMC is rebooting, it is On
I don't see that as a benefit. You can either talk to the BMC or you cannot. Doesn't matter if it is powered on, off, rebooting, etc.
I could be convinced of a BMC state that is 'Available'

MPK  10:31
So there can be a difference between "on" and "reachable".
Not a state, but a "flag".

MJ  10:31
Yes, but we don't know which it is if it is On and Unreachable. (edited) 

MPK  10:32
Right.  But we can say a component is "on/off/ and "reachable/not reachable"
That could be useful info.

ADN  10:33
it seems like a BMC would always be ‘on’ or ‘undefined’

MPK  10:33
Mt. BMCs can be off.

ADN  10:33
they can?

MJ  10:33
Please don't use On with the BMC. It confuses people.
If the slot power is Off the BMCs are Off

MPK  10:33
Sure.  If a node blade is powered off the node card/bmcs are off too, correct?

MJ  10:34
Also if the Outlet pair for a node in an iPDU is Off the BMC would be off

MPK  10:34
Dat too.

ADN  10:35
I think that ‘confusion’ is a real thing to be concerned about; i just think that a standard model that applied to all hardware would be more useful; since we are abstracting the power state based on our knowledge

MPK  10:35
I agree.   Consistency.

MJ  10:36
There is a Chassis power state and status:
ncn-m001:~ # curl -sk -u root:$PASSWD https://x3000c0s22b0/redfish/v1/Chassis/1 | jq '.PowerState,.Status'
"On"
{
  "Health": "OK",
  "State": "Enabled"
}

MPK  10:36
And, if the admin gets a response back that says "you can't do that", I think that's better than having to know what can be powered  or not, assuming there is a clear error message.

MJ  10:36
But we don't report that for River

MPK  10:37
We could either say it's "on" if any thing in the cabinet pings, or "this info is not available", or some such.

MJ  10:38
Please use a word other than On, that confuses people. I am not joking here. People will think they can turn it Off because it is On.
Available and Unavailable would be good.
Or Reachable and Unreachable

MPK  10:38
I'd rather an admin be able to ask for something like river cabinet power and be told "that makes no sense, idjit" than not even allowing them to do it because it wouldn't make sense.

ADN  10:38
ok; we can find a different word probably. Im all for unambigious language (edited) 

MJ  10:39
If something is On that indicates that you can turn it Off.

MPK  10:39
Yup, understood.

ADN  10:40
I like the word available or reachable… I think it implies the ‘hey we can talk to it’ without saying ‘you can mess with it’

MPK  10:40
:+1:

MJ  10:42
We could get fancy if we wanted to. If the slot/outlet power is on but we can't talk to the BMC we could indicate that.

MPK  10:43
Bear in mind that talking to the PDUs in river is dog slow.

ADN  10:43
I think the question becomes do we have ‘one field’ that represents all these or do we return multiple.   If we want to organize this by a single ‘power’ state - eg. on, or off, or unreachable; I think we have to do the former.

MJ  10:43
I think the BMC state should be a different API from the hardware power states.

ADN  10:44
that seems confusing to me… the BMC is hardware

MJ  10:44
But the BMC state we are returning is not a hardware power state. It is a connectivity state.

ADN  10:45
Thats an interesting point.
Id like to see if we can come up with a unified model

MPK  10:45
So BMC types have reachable/unreachable, nodes, etc. are on/off/etc.

ADN  10:45
this is our chance to reset the abstraction layer and become the interface for all power

MPK  10:46
Would my suggestion do the trick?

MJ  10:46
But lets not hem ourselves into requiring a unified model.

ADN  10:47
MPK; quite possibly.
MJ; I agree we dont HAVE to; but if we can, I think that would be good

MPK  10:47
Anything that can be turned on/off would have an off/on/etc state, anything that can't would be reachable/unreachable (or n/a for stuff that has no power or connectivity if we handle anything like that).

MJ  10:47
Just doesn't seem right to have connectivity state returned at the same time as physical power state

MPK  10:47
Why not?

ADN  10:47
but what about the PDU case… where the outlet is on, but the thing aint reachable.

MPK  10:48
It's one or the other depending on the component type.

MJ  10:48
Well if all of my power states are properly returned, I could care less about the connectivity state of the BMCs, because they are working.

ADN  10:48
thats a single case though…

MJ  10:48
That is the majority case.

MPK  10:48
PDU outlets, another case.... :disappointed:

ADN  10:48
right

MPK  10:49
Well, but wouldn't those be on/off and not reachable/unreachable?
I think that still fits my para-diggum

MJ  10:50
If the BMC is Unreachable the node would be Unknown/Unavailable

ADN  10:50
thats not actually true…
a node can be up without its BMC being online… the GBs do it all the time

MPK  10:50
Well, it kind of is true.
It's not known if it can be controlled via the BMC.

MJ  10:50
But we talk to the BMC to determine power state.

ADN  10:50
right

MPK  10:51
so you'd have to determine it's state by other means.

MJ  10:51
We could make a call to the OS to see if it is up. Or we could use the HB.

MPK  10:51
HSM.
Ready == on.
But there are windows where that's wrong too.
Node power dies, BMC is dead, HB stops, and someone asks for a power state 5 seconds after it all happens.   HSM says READY but it's really dead.
Best effort I guess.
Or just say "unknown".
:slightly_smiling_face:

MJ  10:53
That would start mixing logical states with physical states, :face_vomiting:
:face_vomiting:
1

10:53
That is where we are today, it confuses people. :wink:

MPK  10:53
How does it mix logical with physical?

MJ  10:53
If you are asking HSM for power state.

MPK  10:54
We'd just use HSM as a back-up plan.   We can infer the power state of a node by asking HSM.   But it won't always be correct.   So maybe just saying "Unknown" is OK when a BMC is unreachable.
Unknown being from a pure HW perspective.
Maybe using other means if a BMC is down is a future enhancement.
:+1:
1


MJ  10:56
Let me ask this, what would be the benefit of returning BMC state Unavailable, Node state On?

ADN  10:56
it would tell me to back off the GBs

MPK  10:56
Not sure there is one, as long as it's clear that PCS is HARDWARE state only.

ADN  10:57
can you clarify your last intent on that MPK?
like not software state: eg the node is running an OS?

MPK  10:58
As long as users know that what they are asking of PCS is hardware derived state (physical not logical) then it seems acceptable to say Unknown if a BMC is dead.

MJ  10:58
The returning of BMC connectivity state I would expect to be an Admin CLI action only.
A service only cares about the specific xnames they are targeting.

MPK  10:59
"PCS uses hardware to determine power state of hardware.  If the hardware used in making such a determination is unreachable, then downstream components' states cannot be ascertained."
Embossed in the admin guide using 80's era prismatic foil.

MJ  11:00
That is the behavior of CAPMC today. We just don't use the proper adjective in all cases.
Though, (now for me to flip flop), admins do check both sat status and cray capmc get_xname_status and complain when the states are not the same (On/Off mainly). (edited) 

ADN  11:02
b/c sat status uses HSM right?

MPK  11:02
Yes.

MJ  11:02
But, CAPMC is capable of reporting the HSM state as well. And when you do get_node_status, you actually get the HSM state.

MPK  11:02
That's due to edge cases like I described above.
But we don't wanna do that right?

ADN  11:03
right

MJ  11:03
I would rather not. Leave PCS as hardware state only.

MPK  11:03
Yup.

MJ  11:03
What if HSM only reports Ready and NotReady?

ADN  11:03
I think standby is valid?
maybe standby == notready?

MJ  11:04
Hmmm

MPK  11:04
All HSM states are 'valid'.   We don't wanna poke that bear do we?

MPK  11:04
too much depends on HSM for all states.

1 reply
Today at 11:05View thread

MJ  11:04
There is a difference between NotReady because we are Off and Standby because HBs stopped flowing

MPK  11:04
yes.

MJ  11:04
But is it a big enough difference?

MPK  11:05
YUGE.

ADN  11:05
are we thinking PCS will listen to HBs? or just the RF events?

MPK  11:05
I'm thinking neither.

MJ  11:05
I vote just RF events

MPK  11:06
For best accuracy, (just spit-balling here) maybe we should periodically poll the HW.   Maybe skip the poll if we just powered something up and verified it.
11:06
RF events seems to be flaky at least in HSM today.

MJ  11:06
A RF Off event can come in any time.
Yes, they are flakey, and network issues cause subscriptions to disappear in Mountain

MPK  11:06
Ya, maybe we do both.   I'm thinking polling for insurance.

MJ  11:07
I am OK with that.

MPK  11:07
Eventing for immediacy.

ADN  11:08
I agree
… whats the reason to say no to HB?
11:08
not saying we should have it; just curious

MPK  11:09
That's logical state.

MJ  11:09
It would help us to know the node is On, but the node could be On and not heartbeating.
And what MPK said

ADN  11:10
I know we use it to derive a logical state; but its not really logical in an of itself…
The lack of a heartbeat isnt probably good enough; but wouldnt the presence of a heartbeat be informative? would that reduce our need to poll in the back end?

MJ  11:11
HSM uses HB to know if it needs to poll or not. If a node stop HBing it waits for a short time for a RF off event, then it starts to poll the RF ep.

MPK  11:12
But that is only useful if a BMC is unreachable.   Hopefully that's not often.