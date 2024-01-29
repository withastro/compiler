#!/bin/bash

set -euo pipefail

TARGET=wasm32-unknown-unknown
BINARY=target/$TARGET/release/ts_parser.wasm

cargo build --target $TARGET --release
wasm-strip $BINARY
mkdir -p dist
cp $BINARY dist/ts_parser.wasm
mkdir -p ../../cmd/astro-wasm/ts-parser
cp dist/* ../../cmd/astro-wasm/ts-parser
# mkdir -p www
# wasm-opt -o www/ts_parser.wasm -Oz $BINARY
# ls -lh www/ts_parser.wasm
