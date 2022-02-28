#!/bin/bash

# MIT License
#
# (C) Copyright [2020-2022] Hewlett Packard Enterprise Development LP
#
# Permission is hereby granted, free of charge, to any person obtaining a
# copy of this software and associated documentation files (the "Software"),
# to deal in the Software without restriction, including without limitation
# the rights to use, copy, modify, merge, publish, distribute, sublicense,
# and/or sell copies of the Software, and to permit persons to whom the
# Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included
# in all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
# THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
# OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
# ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
# OTHER DEALINGS IN THE SOFTWARE.

maxRetries=50
attempt=0
SMURL=$SMS_SERVER

if [ -z $SMS_SERVER ]; then
	SMURL="http://cray-smd:27779"
fi

echo "========================================================================"
echo "Component Discovery check started.  Expect this to take about 5 minutes."
echo "========================================================================"
echo " "

while [ 1 -eq 1 ]; do
	attempt=`expr $attempt + 1`
	echo "Discovery check ${attempt} of ${maxRetries}..."
	echo "   $discovered of $ncomp components discovered..."

	if [ $attempt -ge $maxRetries ]; then
		echo "DISCOVERY ATTEMPTS EXHAUSTED, GIVING UP."
		break
	fi

	curl -s -D /tmp/hout ${SMURL}/hsm/v2/Inventory/RedfishEndpoints > /tmp/rfep.json

	if [ $? -ne 0 ]; then
		echo "curl to SMD failed."
		cat /tmp/hout
		cat /tmp/rfep.json
		sleep 10
		continue
	fi

	ncomp=`cat /tmp/rfep.json | jq .RedfishEndpoints[].DiscoveryInfo.LastDiscoveryStatus | wc -l`
	if [ $? -ne 0 ]; then
		## something very wrong, this really can't fail unless jq is missing
		echo "cat/jq failed!!!"
		exit 1
	fi

	nComp=`cat /tmp/rfep.json | jq .RedfishEndpoints[].DiscoveryInfo.LastDiscoveryStatus | wc -l`
	if [ $nComp -eq 0 ]; then
		echo "Warning: No components in HSM yet."
		sleep 10
		continue
	fi

	discovered=0
	ok=1
	for cc in `cat /tmp/rfep.json | jq .RedfishEndpoints[].DiscoveryInfo.LastDiscoveryStatus | sed 's/"//g'`; do
		if [ "${cc}" != "DiscoverOK" ]; then
			ok=0
		else
			discovered=`expr $discovered + 1`
		fi
	done

	if [ $ok -eq 1 ]; then
		echo ">>> DISCOVERY COMPLETE.<<<"
		exit 0
	fi

	sleep 10
done

# If we got here, discovery never completed

echo ">>> DISCOVERY FAILED TO COMPLETE!<<<"
exit 1

