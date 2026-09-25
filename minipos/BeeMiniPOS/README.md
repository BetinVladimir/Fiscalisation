# BeeMiniPOS

Expo React Native приложение.

## Запуск
```bash
npm install
npm run start
```

<!-- deployment-and-store-listing:start -->
## Deployment and store listing

Canonical deployment and store metadata for **BeeMiniPOS**. Store links identify the configured product pages; they may remain unavailable publicly while a listing is in draft or internal testing.

| Environment | Web | iOS bundle ID | App Store | Android package | Google Play |
| --- | --- | --- | --- | --- | --- |
| Production | [minipos.beeloy.org](https://minipos.beeloy.org) | `org.beeloy.beeminipos.prod` | [App Store](https://apps.apple.com/app/id6814689857) | `org.beeloy.beeminipos.prod` | [Google Play](https://play.google.com/store/apps/details?id=org.beeloy.beeminipos.prod) |
| Demo | [demo-minipos.beeloy.org](https://demo-minipos.beeloy.org) | `org.beeloy.beeminipos.demo` | [App Store](https://apps.apple.com/app/id6814689862) | `org.beeloy.beeminipos.demo` | [Google Play](https://play.google.com/store/apps/details?id=org.beeloy.beeminipos.demo) |

### Deployment commands

- Production: `npm run deploy:web`
- Demo: `npm run deploy:web-demo`
- Cloudflare configuration: [wrangler.jsonc](./wrangler.jsonc)

### Icons

The icon represents **mobile point of sale**, with the small Beeloy bee mark at the lower-right side.

- Application and iOS icon: [store/icon.png](./store/icon.png)
- Android adaptive foreground: [store/adaptive-icon.png](./store/adaptive-icon.png)
- App Store artwork, 1024 × 1024: [store/graphics/app-store-icon-1024.png](./store/graphics/app-store-icon-1024.png)
- Google Play artwork, 512 × 512: [store/graphics/google-play-icon-512.png](./store/graphics/google-play-icon-512.png)
- Editable vector source: [store/graphics/icon.svg](./store/graphics/icon.svg)

### Store description (en-US)

**Title:** BeeMiniPOS

**Short description:** A compact sales workspace connected to BeeFiscal.

**Full description:**

> BeeMiniPOS provides a compact point-of-sale workspace for businesses using BeeFiscal. Select products, build a cart and follow the sale through the connected fiscal workflow.
>
> Work with the register and operator assigned to your account. Device connectivity, receipt processing and available payment actions depend on your BeeFiscal setup and compatible equipment.
>
> An account and access to the corresponding service are required. Features depend on your permissions and the services enabled by your organization.

Source: [store/listing.en-US.json](./store/listing.en-US.json). The same copy is used for production and demo unless a store-specific override is documented later.
<!-- deployment-and-store-listing:end -->
