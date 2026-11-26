// Live updates: a WebSocket on /api/events tells us what other people (and
// our other tabs) changed, and we invalidate the matching queries so
// TanStack Query refetches what is on screen.
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { request } from './client';
import { keys } from './hooks';

/** One committed change, as sent by the server (see ChangeEvent in docs/openapi.yaml). */
interface ChangeEvent {
  type: 'project' | 'tags' | 'issue' | 'comments' | 'attachments';
  project: string;
  issue?: number;
}

/** Close code the server uses once the session or API key is no longer valid. */
const closeUnauthorized = 4401;

const minRetryMs = 1_000;
const maxRetryMs = 30_000;

function applyChange(qc: QueryClient, ev: ChangeEvent): void {
  const key = ev.project;
  const invalidate = (queryKey: readonly unknown[]) => void qc.invalidateQueries({ queryKey });

  switch (ev.type) {
    case 'project':
      invalidate(keys.projects);
      invalidate(keys.project(key));
      break;
    case 'tags':
      invalidate(keys.tags(key));
      invalidate(keys.issues(key));
      invalidate(['issue', key.toUpperCase()]);
      break;
    case 'issue':
      invalidate(keys.issues(key));
      invalidate(keys.tags(key)); // updates may create tags
      if (ev.issue) invalidate(keys.issue(key, ev.issue));
      break;
    case 'comments':
      if (ev.issue) invalidate(keys.comments(key, ev.issue));
      break;
    case 'attachments':
      if (ev.issue) invalidate(keys.attachments(key, ev.issue));
      break;
  }

  // Every change writes an audit entry.
  invalidate(['auditLog']);
}

function isChangeEvent(v: unknown): v is ChangeEvent {
  return typeof v === 'object' && v !== null && typeof (v as ChangeEvent).type === 'string' && typeof (v as ChangeEvent).project === 'string';
}

/**
 * Keeps a WebSocket to /api/events open while enabled, reconnecting with
 * exponential backoff. After a reconnect every query is invalidated,
 * because changes made while disconnected were missed.
 */
export function useLiveUpdates(enabled: boolean): void {
  const qc = useQueryClient();

  useEffect(() => {
    if (!enabled) return;

    let socket: WebSocket | undefined;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let retryMs = minRetryMs;
    let stopped = false;
    let connectedBefore = false;

    const url = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/events`;

    const connect = () => {
      const ws = new WebSocket(url);
      socket = ws;

      ws.onopen = () => {
        retryMs = minRetryMs;
        if (connectedBefore) void qc.invalidateQueries();
        connectedBefore = true;
      };

      ws.onmessage = (msg) => {
        if (typeof msg.data !== 'string') return;

        let data: unknown;
        try {
          data = JSON.parse(msg.data);
        } catch {
          return;
        }

        if (isChangeEvent(data)) applyChange(qc, data);
      };

      ws.onclose = (ev) => {
        if (stopped) return;

        // Signed out or session expired: any API call now answers 401 and
        // sends the browser to sign in.
        if (ev.code === closeUnauthorized) void request('GET', '/api/me').catch(() => undefined);

        retryTimer = setTimeout(connect, retryMs * (0.5 + Math.random() / 2));
        retryMs = Math.min(retryMs * 2, maxRetryMs);
      };
    };

    connect();

    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      socket?.close(1000);
    };
  }, [enabled, qc]);
}
