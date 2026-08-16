// Package asus provides a client and HTML parser for ASUS Careers
// (https://recruit.asus.com).
//
// # Filter options
//
// /Jobs/GetCities is the site's only JSON endpoint, and it serves cities
// alone. The category, country and experience options have no endpoint
// behind them: they exist only as the /Jobs search form's own controls, so
// [Client.Search] reads them off the page it already fetched and returns them
// on [SearchResponse] rather than this package compiling a table of its own.
//
// That is not only a matter of drift. REQ_TYPEs_Prefix takes the category's
// localized label as its value, and the site is bilingual — a session that
// visited /Home/SetLanguage?culture=en-US submits "Research and Development"
// where a zh-TW one submits "研究發展". A table compiled into this package
// would be right for one locale and silently wrong for the other; the form
// is right for whichever locale served it.
//
// Silently is the operative word: the board drops a REQ_TYPEs_Prefix or
// WORK_EXP value it does not recognize and answers with the whole board, so a
// stale value reads as an unfiltered result set rather than an error. An
// unknown Location, in contrast, matches nothing.
//
// # Country codes
//
// Country codes follow ISO 3166-1 alpha-2 (e.g. TW, US, JP, CN), except
// Slovenia which uses "SL" instead of standard "SI". The list is a master
// table of every country ASUS operates in, not the countries currently
// hiring, so a valid code can legitimately return zero jobs.
package asus
