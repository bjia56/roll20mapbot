#!/bin/bash
xvfb-run --auto-servernum --server-args='-screen 0 1280x720x24' roll20mapbot $@