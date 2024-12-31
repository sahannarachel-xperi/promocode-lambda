package models

type RedemptionRecord struct {
    BusinessDate        string `json:"business_date"`
    CampaignCode       string `json:"campaign_code"`
    VoucherCode        string `json:"voucher_code"`
    RedemptionCode     string `json:"redemption_code"`
    SubscriptionStart  string `json:"subscription_start_dtm"`
}