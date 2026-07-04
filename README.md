# wow-server-ping

| **🇬🇧 English** | [🇷🇺 Русский](README.ru.md) |
| :-: | :-: |

Ping tool for World of Warcraft 335a servers. Can correctly measure ping with servers behind proxy.

![console usage](./images/console.png)

Definitions:

- `Conn` - mean connect time to game server in milliseconds
- `Ping` - mean ping time to game server in milliseconds
- `±` - mean deviation of `Conn` and `Ping`
- `T1` - timeouts during initial TCP connection
- `T2` - timeouts after T1 and until recieving first server message
- `T3` - timeouts after T2 and until recieving second server message
- `E` - errors

It can work as a Prometheus metrics exporter and display graphics in Grafana:

![grafana usage](./images/grafana.png)

## Usage

### Downloads

For Windows you can find builds on the [Release page](https://github.com/egoroof/wow-server-ping/releases/latest). Open an issue if you need another OS builds.

### Realm list

If you are interested in `WoW Circle 3.3.5a` you don't need to extract realm list - it's already included in the build. You can skip this step.

You will need to extract realm list first. Wow servers can give you realm list only after login, so you will have to enter your username and password. This project comes with `realmlist.exe` utility, which logins to WoW server similar real WoW game client and save realm list to `servers` folder.

Run with your user and server host:

```shell
realmlist.exe user@host
```

If you worry about your credentials you can also run Wireshark, login in your WoW client and extract realmlist yourself.

### Ping

Simple example, which  loads realm list from `servers/logon.wowcircle.me.json` file, sends ping requests and print statistics every 30 seconds:

```shell
wow-ping.exe -servers logon.wowcircle.me
```

You can filter servers by regexp with `-filter` option:

```shell
wow-ping.exe -servers logon.wowcircle.me -filter "x4"
```

Windows builds comes with some `.bat` files which you can use or make similar for you.

### Available settings

| Flag | Default | Description |
|---|---|---|
| `-servers` | `logon.wowcircle.me` | Servers config from `servers` folder |
| `-port` | - | Listen port for Prometheus metrics |
| `-timeout` | `1s` | Ping timeout |
| `-interval` | `1s` | Sleep time between requests |
| `-stats-interval` | `10s` | How often stats should be printed to console |
| `-stats` | - | How many stats to display before exit |
| `-filter` | - | Regexp for filter servers by name |

### Ping process

#### Behind proxy

1. You -> establishing TCP connection -> Proxy
2. Proxy -> establishing TCP connection -> Server
3. Server -> packet `SMSG_AUTH_CHALLENGE` -> Proxy -> You
4. You -> packet `CMSG_AUTH_SESSION` -> Proxy -> Server
5. Server -> packet `SMSG_AUTH_RESPONSE` -> Proxy -> You

Сonnection time (`Conn`) measured from step 1 and server ping (`Ping`) from steps 4 - 5.

Timeouts can be helpful for debugging packet losses. There are 3 types of timeouts:

- `T1` - if happen in step 1 (you - proxy)
- `T2` - if happen in steps 2 - 3 (proxy - server)
- `T3` - if happen in steps 4 - 5 (you - server)

#### Without proxy

1. You -> establishing TCP connection -> Server
2. Server -> packet `SMSG_AUTH_CHALLENGE` -> You
3. You -> packet `CMSG_AUTH_SESSION` -> Server
4. Server -> packet `SMSG_AUTH_RESPONSE` -> You

Connection time (`Conn`) measured from step 1 and server ping (`Ping`) from steps 3 - 4.

Timeouts:

- `T1` - if happen in step 1
- `T2` - if happen in step 2
- `T3` - if happen in steps 3 - 4

## Antivirus reaction

Some antivirus software can detect malware (false positive) in downloaded Windows release and block download. You can add an exception and try to download it again. This tool doesn't have any malware. You can check source code and compile it yourself with golang. Also you can scan it with VirusTotal.
