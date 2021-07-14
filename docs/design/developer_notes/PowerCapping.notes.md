## Developer Archeology
*How we got to here...*

Like all things in the PCS space, we got here after much consideration and discussion. There were many core terms that needed definition and we explored the relationship between how things had been done in the past and how they could be done in the future to better align with our microservices architecture.

### Notes -> Might be useful; but don't going digging too hard here gentle reader! 

 * *The final authority on what our software does is the code!*
 * *The final authority of what the software should do is the swagger and design doc!*
 * *The reasonings and how we got to where we are is in the notes*

what is the relationship between 'power capping' 'power scaling (aka ramp limiting)' and 'c-state/p-state'?

	* `c-state` is the sleep states the CPU can go into; how deep can it go to sleep? the deeper the sleep; the longer the latency to wakeup; but the more power it saves. Typically requires a reboot.
	* `p-state` is the frequency control for the CPU. This does directly impact the power consumption of the node. tradeoff power vs performance. This is a workload manager thing
	* `power capping` is the maximum power limit the entire node (with all components) -> NOT THE BLADE, the NODE -> can use.  This is more likely a site-wide setting. Some sites dont have enough power!
	* `power scaling` (aka ramp limiting) is our concept for how fast things get turned on or off to avoid power spikes (in either direction -> to the site, from the site).