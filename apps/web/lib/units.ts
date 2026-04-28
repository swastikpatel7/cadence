/**
 * Distance / pace formatters and converters. Single source of truth so a
 * future migration (e.g. storing distances in km natively) only has to
 * touch this file.
 *
 * The backend is metric end-to-end: distances arrive as kilometers,
 * paces as seconds-per-kilometer. The toggle only affects display.
 */

export type Units = 'metric' | 'imperial';

export const KM_PER_MI = 1.609344;
export const M_PER_MI = 1609.344;

export function milesToKm(mi: number): number {
  return mi * KM_PER_MI;
}

export function kmToMiles(km: number): number {
  return km / KM_PER_MI;
}

/**
 * Format a distance given in kilometers. Renders 1 decimal under 10
 * units, integer above 10 — matches the existing eyeballed convention
 * across heatmap and recent-activities cards.
 */
export function formatDistance(km: number, units: Units): string {
  if (!isFinite(km) || km < 0) return units === 'imperial' ? '0 mi' : '0 km';
  const value = units === 'imperial' ? kmToMiles(km) : km;
  const formatted = value < 10 ? value.toFixed(1) : value.toFixed(0);
  return `${formatted} ${units === 'imperial' ? 'mi' : 'km'}`;
}

/**
 * Format a distance given in meters (Strava's native unit). Thin wrapper
 * around formatDistance for readability at call sites.
 */
export function formatDistanceMeters(m: number, units: Units): string {
  return formatDistance(m / 1000, units);
}

/**
 * Format pace given in seconds-per-kilometer. Converts to per-mile when
 * units='imperial'. Renders M:SS — drops the hour bucket because no
 * runner paces slow enough for it.
 */
export function formatPace(secPerKm: number, units: Units): string {
  if (!isFinite(secPerKm) || secPerKm <= 0) return '—';
  const secPerUnit = units === 'imperial' ? secPerKm * KM_PER_MI : secPerKm;
  const total = Math.round(secPerUnit);
  const min = Math.floor(total / 60);
  const sec = total % 60;
  const suffix = units === 'imperial' ? '/mi' : '/km';
  return `${min}:${sec.toString().padStart(2, '0')}${suffix}`;
}

/**
 * The volume slider stores miles regardless of toggle (matches the
 * existing `weekly_miles_target` column). This helper produces the
 * label users see ("32 mi/wk" or "52 km/wk") given the canonical miles.
 */
export function formatWeeklyVolume(miles: number, units: Units): string {
  if (units === 'imperial') {
    return `${Math.round(miles)} mi/wk`;
  }
  return `${Math.round(milesToKm(miles))} km/wk`;
}

/** Short label used inside chips and dense readouts. */
export function unitShort(units: Units): 'mi' | 'km' {
  return units === 'imperial' ? 'mi' : 'km';
}
