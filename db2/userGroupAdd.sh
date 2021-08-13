#!/bin/bash
groupadd -g 2000 db2iadm1
groupadd -g 2001 db2fadm1
useradd -m -g db2iadm1 -d /home/db2inst1 db2inst1
useradd -m -g db2fadm1 -d /home/db2fenc1 db2fenc1
/opt/ibm/db2/V11.5/instance/db2icrt -u db2inst1 db2inst1
