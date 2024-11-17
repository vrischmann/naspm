# naspm

Dead simple "power management" for my NAS.

*Note*: this is archived because I don't have a use for it anymore as my NAS is now always on.

# Overview

I would like to keep my NAS shut down when not needed, but since I use it remotely I can't always start it by pressing a physical button.

Something already exists for this, it's called [Wake-on-LAN](https://en.wikipedia.org/wiki/Wake-on-LAN) and fortunately my NAS supports it. This only works when connected on the LAN and I regularly need my NAS when I'm outside the house, so we need to somehow have a device that is connected both to the LAN and the internet and ask _that_ device to send the WoL packet.

There are multiple ways to do this but my solution was to leverage [Tailscale](https://tailscale.com): almost every device I have joins my personal tailnet and I can connect to any device from anywhere. On that tailnet I have a router running Linux which is always on and so I can use it to send the WoL packet.

So this covers _waking up_ the NAS if it's shutdown, but what about _shutting it down_ ? Well this is easy: just make it execute `poweroff` somehow. My NAS is also part of my tailnet and I can trivially connect to it to power it off.

# What this is

This program is an implementation of what I described above, tailored made for me so that I can wake up and shutdown my NAS with a simple UI usable on my phone.

There's a single binary that is intended to be run as a background service on each device. It provides the HTTP APIs necessary for everything to work.

There are three modes in which it can work:
* `sleeper` mode
* `waker` mode
* `ui` mode

One particularity of this program is that it uses the [tsnet](https://pkg.go.dev/tailscale.com/tsnet) package to create dedicated tailscale machines and then only exposes a HTTP server on the tailscale interface.

## Sleeper mode

This is the mode that must be run on the NAS. It provides a single HTTP endpoint that calls `poweroff`.

## Waker mode

This is the mode that must be run on the router. It provides a single HTTP endpoint that calls `wol` to send the WoL packet.

## UI mode

This is the mode that must be run on my server. It provides a dead simple form with two buttons: "Wake up" and "Sleep". These buttons will trigger an HTTP request to the endpoints I talked about above.

The following diagram shows the final deployment and request flow: ![Untitled-2022-09-22-1437](https://github.com/vrischmann/naspm/assets/1916079/b8740ef8-5ec7-4fe9-bd09-486d4ccec53f)
