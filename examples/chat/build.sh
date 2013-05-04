#!/usr/bin/env bash

go get
dart2js -oweb/chat.dart.js web/chat.dart
