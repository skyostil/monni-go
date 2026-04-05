#!/bin/sh
# Status display front-end for the Kindle (tested on Kindle Touch).
#
# Dependencies:
# - ImageMagick
# - curl
# - fbink
# - eips
# - lipc-set-prop

#set -e

setup() {
    # Suspend the system UI.
    kill -STOP $(pgrep cvm)
    kill -STOP $(pgrep pillowd)
    kill -STOP $(pgrep blanket)
    lipc-set-prop com.lab126.powerd preventScreenSaver 1
    echo powersave >/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor
}

cleanup() {
    # Resume the system UI.
    kill -CONT $(pgrep cvm)
    kill -CONT $(pgrep pillowd)
    kill -CONT $(pgrep blanket)
    exit 0
}

shuf() {
  awk 'BEGIN {srand(); OFMT="%.17f"} {print rand(), $0}' "$@" | sort -k1,1n | cut -d ' ' -f2-;
}

render() {
    for i in 1 2 3 4 5; do
        ./monni-arm && break
    done

    # Pick a random background image.
    #bg=$(ls -1 images/bg/*.jpg | shuf | head -1)
    #convert "$bg" out.png -composite out-comp.png
    #convert "out-comp.png" -channel R -alpha off "out-bw.png"

    # Clear the screen and display the black and white image.
    ./fbink -k --invert
    sleep 1
    ./fbink -k
    sleep 1
    ./fbink -i "out.png"

    #rm -f "out-bw.png"
    #rm -f "out-comp.png"
    rm -f "out.png"
}

get_battery_level() {
    powerd_test -s | grep -o -e 'Battery Level: [0-9]*' | cut -d ' ' -f3
}

enable_wifi() {
    lipc-set-prop com.lab126.cmd wirelessEnable 1
    #while ! lipc-get-prop com.lab126.wifid cmState | grep -q CONNECTED; do sleep 1; done
}

disable_wifi() {
    lipc-set-prop com.lab126.cmd wirelessEnable 0;
}

wait_for_internet() {
    count=0
    while ! ping -c 1 -W 1 www.google.com > /dev/null 2>&1; do
        sleep 1
        count=$((count + 1))
        if [ $count -eq 60 ]; then
          return 1
        fi
    done
    return 0
}

# Adapted from https://github.com/davidhampgonsalves/life-dashboard/
sleepy_time() {
    next_refresh=10800  # 3 hours

    echo "$(date): Suspending for $next_refresh sec, battery level $(get_battery_level)%"
    sleep 5
    echo $next_refresh > /sys/devices/platform/mxc_rtc.0/wakeup_enable
    echo "mem" > /sys/power/state
}

setup
trap cleanup EXIT SIGINT

while true; do
    echo "$(date): Waiting for wifi"
    #enable_wifi
    if wait_for_internet; then
        echo "$(date): Rendering"
        render
    else
        echo "$(date): Offline"
        eips -f -c
        eips 16 38 "Offline :("
        disable_wifi
        sleep 30
        enable_wifi
        sleep 30
    fi

    sleepy_time
    #disable_wifi
done

