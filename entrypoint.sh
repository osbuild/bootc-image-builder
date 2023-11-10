#!/bin/bash

/usr/bin/osbuild-deploy-container -store /store -rpmmd /rpmmd -output /output -imageref "$@"
