# Jest
A ReST api for creating and managing FreeBSD jails written in Go.

----------

## Howto ##
**Create a jail**

Call `/jails` with a `POST` request and a JSON body:
```bash
curl -X POST "http://10.0.2.4:8080/jails" –data '{"hostname": "mash", "IPV4Addr": "10.0.2.7", "jailName": "mash", "template": "default"}'
```
Response:
```javascript
{
  "Message": "JailConfig found.",
  "Error": null,
  "Jails": {
    "JailConfig": {
      "AllowRawSockets": "",
      "AllowMount": "",
      "AllowSetHostname": "",
      "AllowSysVIPC": "",
      "Clean": "",
      "ConsoleLog": "",
      "Hostname": "mash",
      "IPV4Addr": "10.0.2.7",
      "JailUser": "",
      "JailName": "mash",
      "Path": "",
      "SystemUser": "",
      "Start": "",
      "Stop": "",
      "Template": "default",
      "UseDefaults": ""
    },
    "JailState": {
      "Name": "mash",
      "Running": false,
      "JID": ""
    }
  }
}
```
**List jails**
Call `/jails` with a `GET` request:
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
      "JailConfig": {
        "AllowRawSockets": "",
        "AllowMount": "",
        "AllowSetHostname": "",
        "AllowSysVIPC": "",
        "Clean": "",
        "ConsoleLog": "",
        "Hostname": "pie",
        "IPV4Addr": "10.0.2.6",
        "JailUser": "",
        "JailName": "pie",
        "Path": "",
        "SystemUser": "",
        "Start": "",
        "Stop": "",
        "Template": "default",
        "UseDefaults": ""
      },
      "JailState": {
        "Name": "pie",
        "Running": false,
        "JID": ""
      }
    },
    {
      "JailConfig": {
        "AllowRawSockets": "",
        "AllowMount": "",
        "AllowSetHostname": "",
        "AllowSysVIPC": "",
        "Clean": "",
        "ConsoleLog": "",
        "Hostname": "mash",
        "IPV4Addr": "10.0.2.7",
        "JailUser": "",
        "JailName": "mash",
        "Path": "",
        "SystemUser": "",
        "Start": "",
        "Stop": "",
        "Template": "default",
        "UseDefaults": ""
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
**Get information about a specific jail**
Call `/jails/{jailName}` with a `GET` request:
```bash
curl "http://10.0.2.4:8080/jails/mash"
```
Response:
```javascript
{
  "Message": "Jail found.",
  "Error": null,
  "Jails": {
    "JailConfig": {
      "AllowRawSockets": "",
      "AllowMount": "",
      "AllowSetHostname": "",
      "AllowSysVIPC": "",
      "Clean": "",
      "ConsoleLog": "",
      "Hostname": "mash",
      "IPV4Addr": "10.0.2.7",
      "JailUser": "",
      "JailName": "mash",
      "Path": "",
      "SystemUser": "",
      "Start": "",
      "Stop": "",
      "Template": "default",
      "UseDefaults": ""
    },
    "JailState": {
      "Name": "mash",
      "Running": false,
      "JID": ""
    }
  }
}
```

----------
