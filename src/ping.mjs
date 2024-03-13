import { createConnection } from 'node:net';
import { SMSG_AUTH_CHALLENGE } from './opcodes.mjs';
import { servers } from './servers.mjs';
import { mean, avg, jitter } from './math.mjs';

const params = {
  n: 4, // REQUEST_COUNT
  t: 1000, // TIMEOUT
};
process.argv.forEach((elem) => {
  const [left, right] = elem.split('=');
  const leftWithoutPrefix = left.slice(1);
  const param = params[leftWithoutPrefix];

  if (param !== undefined && right > 0) {
    params[leftWithoutPrefix] = parseInt(right);
  }
});

const REQUEST_COUNT = params.n;
const TIMEOUT = params.t;

console.log(`Requests limit is ${REQUEST_COUNT}, Timeout is ${TIMEOUT} ms`);

process.on('SIGINT', () => {
  printResults();
  process.exit(0);
});

for (const server of servers) {
  server.tcpOpenDurationResults = [];
  server.serverResponseDurationResults = [];
  server.errors = 0;
  server.timeouts = 0;
}

for (let i = 0; i < REQUEST_COUNT; i++) {
  console.log('');
  console.log(`Request # ${i + 1}`);
  for (const server of servers) {
    const res = await openConnection(server.host, server.port);

    if (res.status === 'success') {
      console.log(
        `${server.name} ${res.tcpOpenDuration} ms / ${res.serverResponseDuration} ms`,
      );
      server.tcpOpenDurationResults.push(res.tcpOpenDuration);
      server.serverResponseDurationResults.push(res.serverResponseDuration);
    } else if (res.status === 'timeout') {
      console.log(`${server.name} timeout`);
      server.timeouts++;
    } else {
      console.log(`${server.name} ${res.status}`);
      server.errors++;
    }
  }
}

printResults();

function printResults() {
  const tcpTable = [];
  const serverTable = [];

  for (const server of servers) {
    tcpTable.push({
      server: server.name,
      avg: avg(server.tcpOpenDurationResults),
      mean: mean(server.tcpOpenDurationResults),
      jitter: jitter(server.tcpOpenDurationResults),
      timeouts: server.timeouts,
      errors: server.errors,
    });
    serverTable.push({
      server: server.name,
      avg: avg(server.serverResponseDurationResults),
      mean: mean(server.serverResponseDurationResults),
      jitter: jitter(server.serverResponseDurationResults),
      timeouts: server.timeouts,
      errors: server.errors,
    });
  }
  tcpTable.sort((a, b) => a.avg - b.avg);
  serverTable.sort((a, b) => a.avg - b.avg);

  console.log('');
  console.log(`TCP open time, ms`);
  console.table(tcpTable);

  console.log('');
  console.log(`Server response time, ms`);
  console.table(serverTable);
}

function openConnection(host, port) {
  return new Promise((resolve) => {
    const tcpOpenStartTime = performance.now();
    let serverResponseStartTime = 0;
    const res = {
      status: 'unknown',
      tcpOpenDuration: 0,
      serverResponseDuration: 0,
    };
    const socket = createConnection(
      {
        port,
        host,
        timeout: TIMEOUT,
      },
      () => {
        serverResponseStartTime = performance.now();
        res.tcpOpenDuration = Math.round(performance.now() - tcpOpenStartTime);

        // таймаут до получения ответа сервера
        setTimeout(() => {
          res.status = 'timeout';
          socket.destroy();
        }, TIMEOUT);
      },
    );

    socket.on('timeout', () => {
      res.status = 'timeout';
      socket.destroy();
    });

    socket.on('data', (serverPacket) => {
      const opcode = serverPacket.readUInt16LE(2);
      if (opcode === SMSG_AUTH_CHALLENGE) {
        res.status = 'success';
        res.serverResponseDuration = Math.round(
          performance.now() - serverResponseStartTime,
        );
      } else {
        res.status = 'invalid opcode';
      }

      socket.destroy();
    });

    socket.on('close', () => {
      resolve(res);
    });

    socket.on('error', (error) => {
      res.status = error.message;
      socket.destroy();
    });
  });
}
