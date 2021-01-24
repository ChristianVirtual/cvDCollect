# cvDCollect

This little (and my first) Go program is used to collect the runtime information of my little farm of distributed computers. 

Mainly for BOINC and FAH. 

The goal was to reduce the need to start multiple instances of the respective front end application (like Boinc Manager) to get a glimpse on what going on with the work units in progress. Ideally be possible to also run on mobile devices or lower performance systems like Raspberry Pi.

The source code and be regular downloaded from GitHub or simply installed via

go run github.com/ChristianVirtual/cvDCollect/

You could need to provide a version of the config.json file containing the names, IP address or hostname, port and remote password. Then the collector is staring this config file will be read and used to start the data collection.

From your web browser of choice you can the call localhost:8080/boinc/all to show the prepared web page.

