// Module instances as edited in forms, and their conversion to and from the API.
import {
  isMultiModule,
  type CatalogRef,
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
  type SimCard,
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

/** The hardware payload as edited: the picked entries keep their names for display. */
export interface HardwareForm {
  manufacturer: CatalogRef | null;
  model: CatalogRef | null;
  serialNumber: string;
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
  hardware: HardwareForm;
  networkHost: { role: NetworkRole; hostname: string };
  purchase: PurchaseForm;
  simCard: SimCard;
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

function hardwareForm(m?: Module): HardwareForm {
  const h = m?.hardware;
  // An unknown entry still keeps its ID, so saving does not drop it.
  const ref = (id: string | null | undefined, named: CatalogRef | null | undefined): CatalogRef | null =>
    id ? (named ?? { id, name: id, deleted: false }) : null;

  return { manufacturer: ref(h?.manufacturerId, h?.manufacturer), model: ref(h?.modelId, h?.model), serialNumber: h?.serialNumber ?? '' };
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
    hardware: hardwareForm(stored),
    networkHost: stored?.networkHost ?? { role: 'client', hostname: '' },
    purchase: {
      supplier: p?.supplier ?? '',
      invoiceNumber: p?.invoiceNumber ?? '',
      price: p?.price ?? '',
      currency: p?.currency ?? '',
      warrantyEnd: p?.warrantyEnd ? p.warrantyEnd.slice(0, 10) : '',
    },
    simCard: stored?.simCard ?? { provider: '', phoneNumber: '', iccid: '' },
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
      return {
        ...base,
        hardware: {
          manufacturerId: f.hardware.manufacturer?.id ?? null,
          modelId: f.hardware.model?.id ?? null,
          serialNumber: f.hardware.serialNumber,
        },
      };
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
    case 'sim_card':
      return { ...base, simCard: f.simCard };
    case 'network_interface':
      return { ...base, networkInterface: interfaceInput(f.networkInterface) };
  }
}

