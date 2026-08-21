# CanePanion

CanePanion is a smart-cane companion application designed to improve the
safety, independence, and communication of visually-impared individuals who use mobility canes.

The system connects cane strap firmware attached to their cane and it is connected to a cloud server so that device activity can
be recorded, safety events can be acted upon, and guardians can stay informed.
Its primary purpose is to provide dependable assistance without taking control
away from the cane user.

## Purpose

CanePanion provides a shared safety platform for cane users and their
guardians. It enables a connected cane to:

- Report falls, obstacles, SOS signals, low battery levels, and device errors.
- Share location samples when required for safety or assistance.
- Exchange voice recordings and assistant responses.
- Notify guardians when an event needs their attention.
- Report battery level, connectivity, firmware version, and general device
  health to the cloud.
- Receive configuration changes, commands, and future firmware updates from the
  cloud.

The application is intended to help cane users move with greater confidence
while giving guardians timely, relevant information during emergencies.

## Server responsibilities

This repository contains the CanePanion cloud server. It is responsible for:

- Authenticating users and connected devices.
- Receiving telemetry from cane firmware.
- Storing sensor events, locations, and audio metadata.
- Creating alerts from safety-related sensor events.
- Delivering notifications to users and guardians.
- Supporting reliable communication when a device reconnects after being
  offline.
- Providing a foundation for device configuration, cloud commands, and secure
  firmware updates.

## Documentation

- [Database schema](docs/database-schema.md)
- [Firmware-cloud API proposal](docs/api.md)
- [Architecture map](docs/architecture.html) (double-click the file, or open it in any browser; no server)
- [Live camera path](docs/cane-camera.html) (cane, server, and phone)
---

# CanePanion（繁體中文）

CanePanion 是一款智慧手杖協作應用程式，旨在提升行動手杖使用者的安全性、
自主性與溝通能力。

本系統將手杖韌體連接至雲端伺服器，讓裝置活動得以被記錄、系統能即時處理
安全事件，並使監護人掌握重要資訊。其核心目的，是在不剝奪手杖使用者自主權
的前提下，提供可靠的協助。

## 目的

CanePanion 為手杖使用者與其監護人提供一個共享的安全平台。連線後的智慧手杖
可以：

- 回報跌倒、障礙物、SOS 求救訊號、低電量及裝置錯誤。
- 在安全或協助需求下分享位置資料。
- 傳送語音錄音並接收助理回覆。
- 在事件需要關注時通知監護人。
- 向雲端回報電池電量、連線狀態、韌體版本及整體裝置健康狀況。
- 從雲端接收設定變更、控制指令，以及未來的韌體更新。

本應用程式旨在協助手杖使用者更有信心地行動，並在緊急情況發生時，讓監護人
能及時取得重要且相關的資訊。

## 伺服器職責

此儲存庫包含 CanePanion 雲端伺服器，主要負責：

- 驗證使用者與連線裝置的身分。
- 接收手杖韌體傳送的遙測資料。
- 儲存感測器事件、位置資料及音訊中繼資料。
- 根據安全相關的感測器事件建立警示。
- 向使用者與監護人傳送通知。
- 支援裝置離線後重新連線時的可靠資料傳輸。
- 為裝置設定、雲端指令及安全韌體更新提供基礎架構。

## 文件

- [資料庫結構](docs/database-schema.md)
- [韌體與雲端 API 提案](docs/api.md)
- [架構地圖](docs/architecture.html)（雙擊檔案，或用任何瀏覽器開啟即可，不需伺服器）
- [即時影像路徑](docs/cane-camera.html)（手杖、伺服器、手機）
