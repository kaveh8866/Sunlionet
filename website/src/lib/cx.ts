export type ClassValue = string | null | undefined | false;

export function cx(...values: ClassValue[]) {
  return values.filter(Boolean).join(" ");
}
