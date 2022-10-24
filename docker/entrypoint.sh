#!/bin/sh

# Suppress warning about the "dubious" user id of mounted files when compared to the docker user "root".
git config --global --add safe.directory '*'

bash dargstack "$@"