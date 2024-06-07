package main

type IPWhiteList struct {
	// TODO: Maybe have an option Allow ALL that would disable the whitelist
	whitelist map[string]bool
}

func populateWhiteList(w *IPWhiteList, iplist []string) {
	for _, ip := range iplist {
		w.whitelist[ip] = true
	}
}

func NewIPWhiteList() *IPWhiteList {
	w := IPWhiteList{
		whitelist: make(map[string]bool),
	}
	return &w
}

func (w *IPWhiteList) Allowed(ip string) bool {
	if _, found := w.whitelist[ip]; !found {
		return false
	}
	return true
}
