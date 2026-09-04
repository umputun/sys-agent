#!/bin/sh

printf '%d\n' "$#"
for arg do
    printf '<%s>\n' "$arg"
done
