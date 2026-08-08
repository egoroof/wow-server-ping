# wow-server-ping

| **English** | [Русский](README.ru.md) |
| :-: | :-: |

Ping tool for World of Warcraft 3.3.5a servers. Correctly measures ping to servers behind a proxy.

![console usage](./images/console.png)

Definitions:

- `Conn` - mean connect time to WoW server in milliseconds
- `Hand` - mean handshake time with WoW server in milliseconds
- `Ping` - mean ping time to WoW server in milliseconds
- `±` - mean absolute deviation of `Conn`, `Hand` and `Ping`
- `T1` - timeouts during initial TCP connection (`Conn`)
- `T2` - timeouts after `T1` and until receiving the first server message (`Hand`)
- `T3` - timeouts after `T2`, during sending a message to the server and until receiving server response message (`Ping`)
- `E` - errors

It can work as a [Prometheus](https://prometheus.io) metrics exporter and display graphics in [Grafana](https://grafana.com/oss/grafana/):

![grafana usage](./images/grafana.png)

## Usage

### Downloads

Builds are available for Windows and Linux on the [Release page](https://github.com/egoroof/wow-server-ping/releases/latest). Open an issue if you need another OS builds.

### Realm list

If you are interested in the `WoW Circle 3.3.5a` server you don't need to extract realm list - it's already included in the build. You can skip this step.

You will need to extract realm list first. Wow servers can give you realm list only after login, so you will have to enter your username and password. This project has an utility, which logins to WoW server similar real WoW game client and save realm list to `servers` folder.

Start `realmlist.bat` on Windows or `realmlist` on Linux. It will ask server host, your username and password.

If you worry about your credentials you can run Wireshark, login in your WoW client and extract realmlist yourself.

### Ping

Simple example, which  loads realm list from `servers/logon.wowcircle.me.json` file, sends ping requests and print statistics every 10 seconds:

```shell
wow-ping.exe logon.wowcircle.me
```

You can filter servers by regexp with `-filter` option:

```shell
wow-ping.exe -filter "x4" logon.wowcircle.me
```

Windows builds comes with some `.bat` files which you can use or make similar for you.

### Available settings

| Flag | Default | Description |
|---|---|---|
| `-port` | - | Listen port for Prometheus metrics |
| `-timeout` | `3s` | Ping timeout |
| `-interval` | `500ms` | Sleep time between requests |
| `-stats-interval` | `10s` | How often stats should be printed to console |
| `-stats` | - | How many stats to display before exit |
| `-filter` | - | Regexp for filter servers by name |

### Ping process

We suppose a WoW server can be behind a proxy or a client can use VPN.
That's why a simple ICMP or TCP ping isn't enough.
We need to send and recieve a packet after handshake established to measure ping correctly.

Network requests during a single ping process:

1. You -> TCP SYN -> Proxy
2. Proxy -> TCP SYN-ACK -> You
3. You -> TCP ACK -> Proxy
4. Proxy -> TCP SYN -> Server
5. Server -> TCP SYN-ACK -> Proxy
6. Proxy -> TCP ACK -> Server
7. Server -> packet `SMSG_AUTH_CHALLENGE` -> Proxy -> You
8. You -> packet `CMSG_AUTH_SESSION` -> Proxy -> Server
9. Server -> packet `SMSG_AUTH_RESPONSE` -> Proxy -> You


Сonnection time (`Conn`) is measured from steps 1 - 2, handshake time (`Hand`) is measured from steps 3 - 7 and server ping (`Ping`) from steps 8 - 9.

Timeouts can be helpful for debugging packet losses. There are 3 types of timeouts:

- `T1` - if happen in steps 1 - 2 (you - proxy)
- `T2` - if happen in steps 3 - 7 (proxy - server)
- `T3` - if happen in steps 8 - 9 (you - server)

## Antivirus reaction

Some antivirus software can detect malware (false positive) in downloaded Windows release and block download. You can add an exception and try to download it again. This tool doesn't have any malware. You can check source code and compile it yourself with golang. Also you can scan it with VirusTotal.
