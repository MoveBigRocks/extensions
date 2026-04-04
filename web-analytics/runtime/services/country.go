package analyticsservices

import analyticsdomain "github.com/movebigrocks/extensions/web-analytics/runtime/domain"

func CountryFromLanguage(acceptLang string) string {
	return analyticsdomain.CountryFromLanguage(acceptLang)
}
