// User-visible strings of the UI chrome. Wizard labels come from the wizard file and are
// resolved with text(). Adding a language = adding a dictionary here and a code to LANGS.

const DICTS = {
  en: {
    "app.title": "drydock",
    "app.subtitle": "Build a furyctl.yaml step by step.",
    lang: "Language",
    "picker.kind": "Provider",
    "picker.version": "Distribution version",
    "picker.versionHelp": "Supported range: {range}",
    "picker.start": "Start",
    "picker.loading": "Downloading the distribution…",
    "nav.back": "Back",
    "nav.next": "Next",
    "nav.review": "Review",
    "preview.title": "furyctl.yaml",
    "preview.empty": "The file appears here as you answer.",
    "preview.templateError": "The wizard template failed. This is a drydock bug; the message is below.",
    "status.idle": "Ready",
    "status.working": "Rendering…",
    "status.ok": "Valid",
    "status.errors": "{n} schema errors",
    "source.value": "Value",
    "source.env": "Environment variable",
    "source.file": "File contents",
    "source.path": "Path",
    "source.http": "URL",
    "source.envName": "VARIABLE_NAME",
    "source.filePath": "./path/to/file",
    "source.url": "https://…",
    "list.add": "Add",
    "list.remove": "Remove",
    "nodes.role.cp": "Control plane",
    "nodes.role.etcd": "etcd",
    "nodes.role.lb": "Load balancers",
    "nodes.role.infra": "Infrastructure workers",
    "nodes.role.workers": "Application workers",
    "nodes.firstIp": "First IP",
    "nodes.fill": "Fill IPs",
    "nodes.override": "This role differs from the defaults",
    "nodes.rowOverride": "Override for this node",
    "nodes.hostname": "Hostname",
    "nodes.mac": "MAC address",
    "nodes.ip": "IP address",
    "nodes.dhcp": "DHCP",
    "review.title": "Review",
    "review.intro": "Check the file on the right, then write it.",
    "review.env": "Environment variables to export",
    "review.file": "Files to prepare",
    "review.path": "Paths resolved at apply time",
    "review.http": "URLs fetched at apply time",
    "review.todo": "Before furyctl apply",
    "review.todo.pki": "Run `furyctl create pki` to populate {pkiPath}.",
    "review.todo.mac": "Fill in the MAC address of: {hosts}.",
    "review.todo.validate": "Run `furyctl validate config` once variables and files are in place.",
    "review.todo.dex": "Complete the Dex connector placeholder under distribution.modules.auth.dex.connectors.",
    "review.blocked": "Fix the schema errors before writing.",
    "review.write": "Write furyctl.yaml",
    "review.written": "Written to {path}. furyctl has stopped; you can close this tab.",
    "review.exists": "The output file already exists. Move it away and restart drydock, or use --output.",
    "error.generic": "Something went wrong: {message}",
  },
};

export const LANGS = Object.keys(DICTS);

let lang = "en";
try {
  const stored = localStorage.getItem("drydock.lang");
  if (stored && DICTS[stored]) lang = stored;
} catch {}

export function getLang() {
  return lang;
}

export function setLang(code) {
  if (!DICTS[code]) return;
  lang = code;
  try {
    localStorage.setItem("drydock.lang", code);
  } catch {}
}

export function t(key, vars = {}) {
  const s = DICTS[lang][key] ?? DICTS.en[key] ?? key;
  return s.replace(/\{(\w+)\}/g, (_, k) => (k in vars ? String(vars[k]) : `{${k}}`));
}

// A wizard Text: {en: "...", it: "..."}. Falls back to English, then to the empty string.
export function text(obj) {
  if (!obj) return "";
  if (typeof obj === "string") return obj;
  return obj[lang] ?? obj.en ?? "";
}
