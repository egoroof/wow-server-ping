import { createConnection } from 'node:net';
import { SMSG_AUTH_CHALLENGE } from './opcodes.mjs';
import { servers } from './servers.mjs';
import { avg, jitter } from './math.mjs';

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

console.log(`Requests limit ${REQUEST_COUNT}`);
console.log(`Timeout ${TIMEOUT} ms`);

process.on('SIGINT', () => {
  printResults();
  process.exit(0);
});

for (const server of servers) {
  server.responseDurations = [];
  server.errors = 0;
  server.timeouts = 0;
}

for (let i = 0; i < REQUEST_COUNT; i++) {
  console.log('');
  console.log(`Request # ${i + 1}`);
  for (const server of servers) {
    const res = await openConnection(server.host, server.port);

    if (res.status === 'success') {
      console.log(`${server.name} ${res.responseDuration} ms`);
      server.responseDurations.push(res.responseDuration);
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
  const serverTable = [];
  let maxNameLength = 0;

  for (const server of servers) {
    if (server.name.length > maxNameLength) {
      maxNameLength = server.name.length;
    }
    serverTable.push({
      name: server.name,
      avg: avg(server.responseDurations),
      jitter: jitter(server.responseDurations),
      timeouts: server.timeouts,
      errors: server.errors,
    });
  }
  serverTable.sort((a, b) => {
    if (a.errors - b.errors !== 0) {
      return a.errors - b.errors;
    }
    if (a.timeouts - b.timeouts !== 0) {
      return a.timeouts - b.timeouts;
    }
    return a.avg - b.avg;
  });

  console.log('');
  console.log(`Response time, ms`);

  for (const server of serverTable) {
    let timeoutStr = '';
    if (server.timeouts > 0) {
      timeoutStr = `; ${server.timeouts} timeouts`;
    }
    let errorStr = '';
    if (server.errors > 0) {
      errorStr = `; ${server.errors} errors`;
    }
    let statsStr = 'unavailable';
    if (server.avg > 0) {
      statsStr = `${server.avg} ± ${server.jitter}`;
    }
    let extraSpaces = '';
    if (maxNameLength - server.name.length > 0) {
      extraSpaces = ' '.repeat(maxNameLength - server.name.length);
    }
    console.log(
      `${server.name}  ${extraSpaces}${statsStr}${timeoutStr}${errorStr}`,
    );
  }
}

function openConnection(host, port) {
  return new Promise((resolve) => {
    let serverResponseStartTime = 0;
    const res = {
      status: 'unknown',
      responseDuration: 0,
    };
    const socket = createConnection(
      {
        port,
        host,
        timeout: TIMEOUT,
      },
      () => {
        serverResponseStartTime = performance.now();

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
        res.responseDuration = Math.round(
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
