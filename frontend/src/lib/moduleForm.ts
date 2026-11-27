// Module instances as edited in forms, and their conversion to and from the API.
import {
  isMultiModule,
  type IPConfig,
  type IPConfigInput,
  type IPv4Mode,
  type IPv6Mode,
  type Module,
  type ModuleInput,
  type ModuleKind,
  type NetworkInterface,
  type NetworkInterfaceInput,
  type NetworkInterfaceType,
  type NetworkRole,
} from '../api/types';

/** One IP family as edited: numbers stay text until submit. */
export interface IPForm<Mode extends string> {
  mode: Mode;
  address: string;
  prefixLength: string;
  gateway: string;
  dnsServers: string[];
}

export interface InterfaceForm {
  name: string;
  mac: string;
  type: NetworkInterfaceType;
  ssid: string;
  nativeVlan: string;
  taggedVlans: string[];
  ipv4: IPForm<IPv4Mode>;
  ipv6: IPForm<IPv6Mode>;
}

export interface PurchaseForm {
  supplier: string;
  invoiceNumber: string;
  price: string;
  currency: string;
  /** yyyy-mm-dd or empty. */
  warrantyEnd: string;
}

/** One module instance as edited. */
export interface ModuleForm {
  /** Stable React key. */
  key: number;
  /** Set for instances already stored, so they keep their identity. */
  id?: string;
  kind: ModuleKind;
  primary: boolean;
  hardware: { manufacturer: string; model: string; serialNumber: string };
  networkHost: { role: NetworkRole; hostname: string };
  purchase: PurchaseForm;
  networkInterface: InterfaceForm;
}

let nextKey = 0;

function ipForm<Mode extends string>(c: IPConfig<Mode>): IPForm<Mode> {
  return {
    mode: c.mode,
    address: c.address ?? '',
    prefixLength: c.prefixLength === null ? '' : String(c.prefixLength),
    gateway: c.gateway ?? '',
    dnsServers: c.dnsServers,
  };
}

function interfaceForm(n?: NetworkInterface): InterfaceForm {
  return {
    name: n?.name ?? '',
    mac: n?.mac ?? '',
    type: n?.type ?? 'wired',
    ssid: n?.ssid ?? '',
    nativeVlan: n?.nativeVlan == null ? '' : String(n.nativeVlan),
    taggedVlans: n?.taggedVlans.map(String) ?? [],
    ipv4: n ? ipForm(n.ipv4) : { mode: 'dhcp', address: '', prefixLength: '', gateway: '', dnsServers: [] },
    ipv6: n ? ipForm(n.ipv6) : { mode: 'off', address: '', prefixLength: '', gateway: '', dnsServers: [] },
  };
}

/** A form for module m, or an empty one of kind. */
export function moduleForm(m: Module | ModuleKind): ModuleForm {
  const stored = typeof m === 'string' ? undefined : m;
  const p = stored?.purchase;

  return {
    key: nextKey++,
    id: stored?.id,
    kind: stored?.kind ?? (m as ModuleKind),
    primary: stored?.primary ?? false,
    hardware: stored?.hardware ?? { manufacturer: '', model: '', serialNumber: '' },
    networkHost: stored?.networkHost ?? { role: 'client', hostname: '' },
    purchase: {
      supplier: p?.supplier ?? '',
      invoiceNumber: p?.invoiceNumber ?? '',
      price: p?.price ?? '',
      currency: p?.currency ?? '',
      warrantyEnd: p?.warrantyEnd ? p.warrantyEnd.slice(0, 10) : '',
    },
    networkInterface: interfaceForm(stored?.networkInterface),
  };
}

/** Text to a number, or null when empty; unparsable text becomes NaN, which the server rejects. */
function optionalNumber(s: string): number | null {
  return s.trim() === '' ? null : Number(s);
}

function ipInput<Mode extends string>(f: IPForm<Mode>): IPConfigInput<Mode> {
  if (f.mode !== 'static') return { mode: f.mode };

  return {
    mode: f.mode,
    address: f.address.trim() || null,
    prefixLength: optionalNumber(f.prefixLength),
    gateway: f.gateway.trim() || null,
    dnsServers: f.dnsServers,
  };
}

function interfaceInput(f: InterfaceForm): NetworkInterfaceInput {
  return {
    name: f.name,
    mac: f.mac,
    type: f.type,
    ssid: f.type === 'wireless' ? f.ssid : null,
    nativeVlan: optionalNumber(f.nativeVlan),
    taggedVlans: f.taggedVlans.map(Number),
    ipv4: ipInput(f.ipv4),
    ipv6: ipInput(f.ipv6),
  };
}

/** A date input value (yyyy-mm-dd) as an RFC 3339 timestamp at midnight UTC, or null. */
export function dateInput(value: string): string | null {
  return value ? `${value}T00:00:00Z` : null;
}

/** The module as sent to the API: only the payload matching its kind. */
export function moduleInput(f: ModuleForm): ModuleInput {
  const base: ModuleInput = { id: f.id, kind: f.kind, primary: isMultiModule(f.kind) && f.primary };

  switch (f.kind) {
    case 'hardware':
      return { ...base, hardware: f.hardware };
    case 'network_host':
      return { ...base, networkHost: f.networkHost };
    case 'purchase':
      return {
        ...base,
        purchase: {
          supplier: f.purchase.supplier,
          invoiceNumber: f.purchase.invoiceNumber,
          price: f.purchase.price.trim() || null,
          currency: f.purchase.currency,
          warrantyEnd: dateInput(f.purchase.warrantyEnd),
        },
      };
    case 'network_interface':
      return { ...base, networkInterface: interfaceInput(f.networkInterface) };
  }
}

