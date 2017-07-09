# Jest
A ReST api for creating and managing FreeBSD jails written in Go.

[![Build Status](https://travis-ci.org/altsrc-io/Jest.svg?branch=master)](https://travis-ci.org/altsrc-io/Jest)
[![Coverage Status](https://coveralls.io/repos/github/altsrc-io/Jest/badge.svg?branch=master)](https://coveralls.io/github/altsrc-io/Jest?branch=master)
[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)

----------

## Jails ##
**Create a jail**

Call `/jails` with a `POST` request and a JSON body:
```bash
curl -X POST "http://10.0.2.4:8080/jails" –data 
'{"hostname": "mash", "IPV4Addr": "10.0.2.7", "jailName": "mash", "template": "default", "useDefaults": true}'
```
Response:
```javascript
{
  "Message": "Jail created successfully",
  "Error": null,
  "JUID": "3254ec98-e683-429a-9849-7e432c24c01b"
}
```
**List jails**

Call `/jails` with a `GET` request. You can see we have 3 jails configured on this host, **pie**, **mash** and **gravy**:
```bash
curl "http://10.0.2.4:8080/jails"
```
Response:
```javascript
{
  "Message": "Jails found.",
  "Error": null,
  "Jails": [
    {
      "Name": "pie",
      "JailConfig": {
        "AllowRawSockets": "0",
        "AllowMount": "0",
        "AllowSetHostname": "0",
        "AllowSysVIPC": "0",
        "Clean": "0",
        "ConsoleLog": "/var/log/jail_${name}_console.log",
        "Hostname": "pie",
        "IPV4Addr": "10.0.2.9",
        "JailUser": "root",
        "JailName": "pie",
        "Path": "/usr/jail",
        "SystemUser": "root",
        "Start": "/bin/sh /etc/rc",
        "Stop": "/bin/sh /etc/rc.shutdown",
        "Template": "default",
        "UseDefaults": true
      },
      "JailState": {
        "Name": "pie",
        "Running": false,
        "JID": ""
      }
    },
    {
      "Name": "gravy",
      "JailConfig": {
        "AllowRawSockets": "0",
        "AllowMount": "0",
        "AllowSetHostname": "0",
        "AllowSysVIPC": "0",
        "Clean": "0",
        "ConsoleLog": "/var/log/jail_${name}_console.log",
        "Hostname": "gravy",
        "IPV4Addr": "10.0.2.8",
        "JailUser": "root",
        "JailName": "gravy",
        "Path": "/usr/jail",
        "SystemUser": "root",
        "Start": "/bin/sh /etc/rc",
        "Stop": "/bin/sh /etc/rc.shutdown",
        "Template": "default",
        "UseDefaults": true
      },
      "JailState": {
        "Name": "gravy",
        "Running": false,
        "JID": ""
      }
    },
    {
      "Name": "mash",
      "JailConfig": {
        "AllowRawSockets": "0",
        "AllowMount": "0",
        "AllowSetHostname": "0",
        "AllowSysVIPC": "0",
        "Clean": "0",
        "ConsoleLog": "/var/log/jail_${name}_console.log",
        "Hostname": "mash",
        "IPV4Addr": "10.0.2.10",
        "JailUser": "root",
        "JailName": "mash",
        "Path": "/usr/jail",
        "SystemUser": "root",
        "Start": "/bin/sh /etc/rc",
        "Stop": "/bin/sh /etc/rc.shutdown",
        "Template": "default",
        "UseDefaults": true
      },
      "JailState": {
        "Name": "mash",
        "Running": false,
        "JID": ""
      }
    }
  ]
}
```
You can also get the information for a specific jail by issue a GET request to `/jails/{jailName}` for example:

    curl "http://10.0.2.4:8080/jails/mash"


**Change the state of a jail**

Call `/jails/{jailName}` with a `PUT` request. For example, to start a jail, you would put the 'Running' state to 'true':
```bash
curl -X PUT "http://10.0.2.4:8080/jails/mash" --data '{"JailState": {"Name": "mash","Running": true}}'
```
Response:
```javascript
  {
  "Message": "Jail state updated.",
  "Error": null,
  "JailState": {
      "Name": "mash",
      "Running": true,
      "JID": "2"
    }
  }
```

You can also update the configuration for the jail the same way.

**Delete a jail**

Call `/jails/{jailName}` with a `DELETE` request:
```bash
curl -X DELETE "http://10.0.2.4:8080/jails/mash"
```
Response:
```javascript
{
  "Message": "Jail deleted.",
  "Error": null,
  ...
}
```

----------

## Templates ##
Templates are jails which serve as a template for the creation of other jails, you can deploy a specific FreeBSD version into a template, configure any global settings such as DNS and then use it to create new jails quickly and easily. 

## Snapshots ##
Snapshots allow you to backup your jails and templates at specific points in time, including the underlying ZFS datasets and the related Jest configuration.

## Config ##
Config is where you can query or update the Jest configuration for a particular agent.
