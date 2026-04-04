package analyticsservices

import analyticsdomain "github.com/movebigrocks/extensions/web-analytics/runtime/domain"

func ClassifySource(utmSource, referrer, propertyDomain string) string {
	return analyticsdomain.ClassifySource(utmSource, referrer, propertyDomain)
}

func ClassifyChannel(utmSource, utmMedium, referrer, propertyDomain string) string {
	return analyticsdomain.ClassifyChannel(utmSource, utmMedium, referrer, propertyDomain)
}
