#!/bin/bash
set -ex

oc delete validatingwebhookconfiguration/vhorizon.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration/mhorizon.kb.io --ignore-not-found
