#!/bin/sh
# Script to initialise docker environment.


# watchlist.txt
if [ ! -f /config/watchlist.txt ]
then
    cp /disk-tool/config-files/watchlist.txt /config/
fi

if [ -e /disk-tool/watchlist.txt ]
then
    rm /disk-tool/watchlist.txt
fi
ln -s /config/watchlist.txt /disk-tool/watchlist.txt


# diskInfo.db
if [ ! -f /config/diskInfo.db ]
then
    cp /disk-tool/config-files/diskInfo.db /config/
fi

if [ -e /disk-tool/diskInfo.db ]
then
    rm /disk-tool/diskInfo.db
fi
ln -s /config/diskInfo.db /disk-tool/diskInfo.db

/disk-tool/disk-tool