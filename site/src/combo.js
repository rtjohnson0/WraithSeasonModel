// Only explicit COMBO selections belong on the picks tab. Comparison estimates
// remain available for analysis without being presented as betting selections.
export function comboGroups(rows = []) {
  return {
    picks: rows.filter(row => row.tier === 'COMBO'),
    comparisons: rows.filter(row => row.tier === 'COMBOWATCH'),
  }
}
