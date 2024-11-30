import Docker from 'dockerode';
import { debounce, groupBy } from 'es-toolkit';
import { readFile, writeFile } from 'node:fs/promises';

const CADDY_GEN_NETWORK = process.env.CADDY_GEN_NETWORK || 'gateway';
const CADDY_GEN_OUTFILE = process.env.CADDY_GEN_OUTFILE || 'docker-sites.caddy';
const CADDY_GEN_DOCKER = safeParseJson(process.env.CADDY_GEN_DOCKER);
const CADDY_GEN_NOTIFY = safeParseJson(process.env.CADDY_GEN_NOTIFY);

const docker = new Docker(CADDY_GEN_DOCKER);
const debouncedCheckConfig = debounce(checkConfig, 1000);
debouncedCheckConfig();
await bindEvents();

function safeParseJson(raw?: string) {
  try {
    return JSON.parse(raw || '');
  } catch {
    // ignore
  }
}

async function generateConfig() {
  const containers = await docker.listContainers({
    filters: {
      network: [CADDY_GEN_NETWORK],
      status: ['created', 'restarting', 'running'],
    },
  });
  const items = containers.flatMap((info) => {
    const raw = info.Labels['virtual.bind']?.trim();
    if (!raw) return [];
    return raw.split(';').map((bindInfo) => {
      const [bind, ...directives] = bindInfo.split('|').map((s) => s.trim());
      const bindParts = bind.split(' ');
      let path = '';
      if (bind.startsWith('/')) {
        path = bindParts.shift()!;
      }
      const [port, ...hostnames] = bindParts;
      const hostDirectives: string[] = [];
      const proxyDirectives: string[] = [];
      directives.forEach((directive) => {
        if (directive.startsWith('host:')) {
          hostDirectives.push(directive.slice(5).trim());
        } else {
          proxyDirectives.push(directive);
        }
      });
      const proxyIp =
        info.NetworkSettings.Networks[CADDY_GEN_NETWORK].IPAddress;
      return {
        hostnames,
        port: +port,
        pathMatcher: path,
        name: info.Names[0].slice(1),
        hostDirectives,
        proxyDirectives,
        proxyIp,
      };
    });
  });
  const groups = groupBy(items, (item) => item.hostnames.join(' '));
  const config = Object.entries(groups)
    .map(([hostnames, group], i) => {
      const hostMatcher = `@caddy-gen-${i}`;
      return [
        `${hostMatcher} host ${hostnames}`,
        `handle ${hostMatcher} {`,
        ...group.flatMap((item) => item.hostDirectives).map((s) => `  ${s}`),
        ...group
          .flatMap((item) => [
            `# ${item.name}`,
            `reverse_proxy ${item.pathMatcher} {`,
            ...item.proxyDirectives.map((s) => `  ${s}`),
            `  to ${item.proxyIp}:${item.port}`,
            `}`,
          ])
          .map((s) => `  ${s}`),
        `}`,
      ].join('\n');
    })
    .join('\n\n');
  return config;
}

function log(...args: unknown[]) {
  console.log(new Date().toLocaleString(), ...args);
}

async function notify() {
  if (!CADDY_GEN_NOTIFY) return;
  log('Notify:', CADDY_GEN_NOTIFY);
  const { containerId, workingDir, command } = CADDY_GEN_NOTIFY;
  try {
    const container = docker.getContainer(containerId);
    const exec = await container.exec({
      Cmd: command,
      WorkingDir: workingDir,
    });
    await exec.start({});
  } catch (err) {
    console.error(err);
  }
}

async function checkConfig() {
  let currentConfig = '';
  try {
    currentConfig = await readFile(CADDY_GEN_OUTFILE, 'utf8');
  } catch {
    // ignore
  }
  const newConfig = await generateConfig();
  if (currentConfig !== newConfig) {
    await writeFile(CADDY_GEN_OUTFILE, newConfig);
    log('Caddy config written:', CADDY_GEN_OUTFILE);
    await notify();
  } else {
    log('No change, skip notifying');
  }
}

async function bindEvents() {
  const stream = await docker.getEvents({
    filters: {
      type: ['container'],
      event: ['start', 'stop'],
    },
  });
  log('Waiting for Docker events...');
  for await (const _raw of stream) {
    // const event = JSON.parse(new TextDecoder().decode(raw as Buffer)) as {
    //   status: 'start' | 'stop';
    //   id: string;
    //   from: string;
    //   Type: 'container';
    //   Actor: {
    //     ID: string;
    //     Attributes: Record<string, string>;
    //   };
    //   scope: 'local';
    //   time: number;
    //   timeNano: number;
    // };
    debouncedCheckConfig();
  }
}
