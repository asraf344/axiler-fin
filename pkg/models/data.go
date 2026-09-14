/*
 * Copyright (c) 2026 peek8.io
 *
 * Created Date: Monday, September 14th 2026, 1:27:03 pm
 * Author: Md. Asraful Haque
 *
 */

package models

import (
	"fmt"
	"math/rand"
	"time"
)

var tenants = []string{
	"alpha",
	"beta",
	"gamma",
}

var owners = map[string][]string{
	"alpha": {
		"Alice Savings",
		"Alpha Trading",
		"Alpha Holdings",
		"Alice Investments",
		"Alpha Finance",
		"Alpha Retail",
		"Alpha Technologies",
		"Alpha Partners",
	},
	"beta": {
		"Bob Enterprise",
		"Beta Industries",
		"Bob Holdings",
		"Beta Trading",
		"Beta Solutions",
		"Beta Capital",
		"Beta Commerce",
		"Bob Investments",
	},
	"gamma": {
		"Gamma Ventures",
		"Gamma Holdings",
		"Gamma Technologies",
		"Gamma Capital",
		"Gamma Investments",
		"Gamma Trading",
		"Gamma Industries",
		"Gamma Partners",
	},
}

var statuses = []string{
	"active",
	"active",
	"active",
	"active",
	"suspended",
	"closed",
}

var currencies = []string{
	"USD",
}

func generateAccounts(count int) map[string]*Account {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	accounts := make(map[string]*Account)

	// Keep separate account counters for each tenant.
	counters := map[string]int{
		"alpha": 0,
		"beta":  0,
		"gamma": 0,
	}

	for i := 0; i < count; i++ {
		tenant := tenants[rng.Intn(len(tenants))]

		counters[tenant]++

		ownerList := owners[tenant]
		owner := ownerList[rng.Intn(len(ownerList))]

		// Generate balance between $1,000 and $500,000.
		balance := float64(rng.Intn(499000)+1000) +
			float64(rng.Intn(100))/100

		// Account created between 1 hour and 180 days ago.
		hoursAgo := rng.Intn(180*24) + 1
		createdAt := time.Now().Add(
			-time.Duration(hoursAgo) * time.Hour,
		)

		account := Account{
			ID: fmt.Sprintf(
				"acc_%s_%03d",
				tenant,
				counters[tenant],
			),
			TenantID:  tenant,
			Owner:     owner,
			Balance:   balance,
			Currency:  currencies[rng.Intn(len(currencies))],
			Status:    statuses[rng.Intn(len(statuses))],
			CreatedAt: createdAt,
		}

		accounts[account.ID] = &account
	}

	return accounts
}
