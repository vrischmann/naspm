# naspm

Dead simple "power management" for my NAS.

## Overview

My NAS is compatible with [Wake-On-Lan](https://en.wikipedia.org/wiki/Wake-on-LAN) and I take advantage to only start it when necessary (mainly when using Plex).

When I'm connected directly in the LAN where it's sitting it's easy, I can just run a command in a terminal and it works fine.

However my NAS is also accessible over Tailscale but in that case I can't wake directly use WOL from my remote device, I need another solution.
My solution is to use my router that's running Linux, always on and also connected to my Tailnet.

The flow is then like this:
* the remote device talks over the Tailnet to the router, it asks it to wake up the NAS
* the router sends the WoL packet over the LAN

This covers waking up the NAS but I also want an easy way to power if off because I don't want to have to SSH on it and run `poweroff` myself.

## What this is

So this program is a really simple HTTP server that provides both things:
* when run on the NAS in `sleeper` mode, a request will poweroff the NAS
* when run on my router in `waker` mode, a request will send the WoL packet and start the NAS

The HTTP endpoints are only available on the Tailnet, I use the [tsnet](https://pkg.go.dev/tailscale.com/tsnet) to do this.
