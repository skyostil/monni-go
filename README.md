# Monni-go

A simple little status display for a jailbroken Kindle tablet. Currently shows
the following information:

 - Items from a Google Keep checklist.
 - Weather forecast from [BOM](https://www.bom.gov.au/).
 - Current weather from [wttr.in](https://wttr.in).
 - Train and bus departures from
   [PTV](https://www.vic.gov.au/public-transport-timetable-api).

I'm using this a 4th generation Kindle Touch. It gets about a month of battery
usage with one update every 3 hours.

## Configuration

Configuration is currently done by editing `monni.go` directly.

For Google Keep:
  - Create a service account that has access to the notes you 
    want to display with the https://www.googleapis.com/auth/keep.readonly
    OAuth2 scope. Save the account information into `service-account.json`.
  - Change `serviceAccountUser` to match your email address.
  - Change the `notes` array to contain the notes you want to display.

For BOM (weather):
  - Update `weatherLocation` to your desired the city/region.
  - Update `bomProduct` to the matching region from
    https://www.bom.gov.au/catalogue/data-feeds.shtml.
  - Update `bomGeohash` to match your desired location. To look up a geohash,
    see https://api.weather.bom.gov.au/v1/locations?search=NNNN where NNNN is
    your postcode. Use the first 6 characters of the returned 7 character geohash.
  - Update `bomProduct` to match your location. See
    https://www.bom.gov.au/catalogue/data-feeds.shtml.

For PTV (transit):
  - Register for an API key at
    https://www.vic.gov.au/public-transport-timetable-api and update
    `ptvDevId` and `ptvApiKey` to match.
  - Update `ptvTrainStopId` and `ptvBusStopId` to match what you want to
    display.

## Building

To build, run `./build-docker.sh` (requires Docker). The build is done through
Docker, because we require an ancient version of Go that still works on the
Kindle kernel.

## Installation and usage

First, copy the required files to your jailbroken Kindle, run `./deploy.sh KINDLE_IP_ADDRESS`.

Then, run `/mnt/us/monni.sh` on the Kindle (ideally in `tmux`). It will suspend
the standard UI to save power and enter a loop that updates the display every 3
hours. Hit `Ctrl-C` to exit and restore the standard UI.

Note that you can also `go run monni` on a PC to generate the rendered output in
`out.png`.

## Dependencies

- The project uses [FBInk](https://www.mobileread.com/forums/showthread.php?t=299110) to control
  the display. There is a prebuilt arm32 binary in the repo for convenience.
- The font is [Merriweather](https://fonts.google.com/specimen/Merriweather).
