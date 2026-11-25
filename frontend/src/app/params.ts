import { useParams } from 'react-router';

/** Returns a route parameter that the route definition guarantees. */
export function useParam(name: 'key' | 'number'): string {
  return useParams()[name] ?? '';
}
