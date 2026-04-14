export type DonationAddress = {
  symbol: string;
  network: string;
  address: string;
};

export const supportConfig = {
  revolutReferralUrl:
    process.env.NEXT_PUBLIC_REVOLUT_REFERRAL_URL ??
    "https://revolut.com/referral/?referral-code=mostafy1un!APR1-26-VR-DE&geo-redirect",
  revolutMaxReward: process.env.NEXT_PUBLIC_REVOLUT_MAX_REWARD ?? "$200",
  revolutQrImagePath: process.env.NEXT_PUBLIC_REVOLUT_QR_IMAGE_PATH ?? "/support/revolut-aff-link-qr-code.png",
  donationAddresses: [
    {
      symbol: "XMR",
      network: "Monero",
      address:
        process.env.NEXT_PUBLIC_DONATION_XMR ??
        "YOUR_XMR_ADDRESS_HERE",
    },
    {
      symbol: "BTC",
      network: "Bitcoin",
      address:
        process.env.NEXT_PUBLIC_DONATION_BTC ??
        "YOUR_BTC_ADDRESS_HERE",
    },
    {
      symbol: "USDT",
      network: "TRC20",
      address:
        process.env.NEXT_PUBLIC_DONATION_USDT_TRC20 ??
        "YOUR_USDT_TRC20_ADDRESS_HERE",
    },
  ] as DonationAddress[],
};

export const primaryQrValue = supportConfig.donationAddresses[0]?.address ?? "";
