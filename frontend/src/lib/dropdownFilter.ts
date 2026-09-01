/* Pure option model + filter logic for the searchable Dropdown menu, kept out
   of the component file so node-based verification scripts can import it
   without a JSX pipeline (same split as kvKeyTree / named-fields). */

export interface DropdownOption {
  value: string;
  label: string;
  icon?: string;
  hint?: string;
}

/* Case-insensitive substring filter for searchable menus. Matches the label,
 * the optional hint, and the raw value — the value is what makes huge model
 * lists usable: users search by model key ("claude-sonnet-4-5-20250929") even
 * when the label is a display title ("Claude Sonnet 4.5"). */
export function filterDropdownOptions(options: DropdownOption[], query: string): DropdownOption[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return options;
  return options.filter(
    (o) =>
      o.label.toLowerCase().includes(needle) ||
      (o.hint ?? "").toLowerCase().includes(needle) ||
      o.value.toLowerCase().includes(needle),
  );
}
