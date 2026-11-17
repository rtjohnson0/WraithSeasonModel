// Only explicit COMBO selections belong on the picks tab. Comparison estimates
// remain available for analysis without being presented as betting selections.
export function comboGroups(rows = []) {
  return {
    picks: rows.filter(row => row.tier === 'COMBO' && !row.research && !row.rotation),
    comparisons: rows.filter(row => row.tier === 'COMBOWATCH' || row.tier === 'COMBORESEARCH' || row.research || !!row.rotation),
  }
}
