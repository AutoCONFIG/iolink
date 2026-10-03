# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: support-expiry.spec.ts >> 支持到期：UTC >> 未编辑到期时间的重复保存保留 UTC 秒数
- Location: e2e/support-expiry.spec.ts:6:5

# Error details

```
Error: expect(received).toEqual(expected) // deep equality

- Expected  - 1
+ Received  + 1

@@ -1,9 +1,9 @@
  Array [
    Object {
      "active": true,
-     "expiresAt": "2099-10-02T05:00:37Z",
+     "expiresAt": "2099-10-02T05:00:00Z",
      "name": "秒数测试成员",
      "role": "support",
      "tenantId": 1,
      "userId": 2,
    },
```

# Page snapshot

```yaml
- generic [ref=f1e1]:
  - generic [ref=f1e3]:
    - complementary [ref=f1e4]:
      - generic [ref=f1e10]:
        - strong [ref=f1e11]: IoLink
        - text: 智慧水产管理平台
      - navigation [ref=f1e12]:
        - link "运营总览" [ref=f1e13] [cursor=pointer]:
          - /url: /dashboard
        - link "养殖场与池塘" [ref=f1e18] [cursor=pointer]:
          - /url: /ponds
        - link "设备管理" [ref=f1e24] [cursor=pointer]:
          - /url: /devices
        - link "产品与物模型" [ref=f1e29] [cursor=pointer]:
          - /url: /products
        - link "报警规则" [ref=f1e34] [cursor=pointer]:
          - /url: /alarms/rules
        - link "报警中心" [ref=f1e39] [cursor=pointer]:
          - /url: /alarms
        - link "系统设置" [ref=f1e46] [cursor=pointer]:
          - /url: /system
        - link "组织与成员" [ref=f1e51] [cursor=pointer]:
          - /url: /tenants
      - generic [ref=f1e58]: 服务状态待后端健康检查
    - generic [ref=f1e61]:
      - generic [ref=f1e62]:
        - generic [ref=f1e63]:
          - button "收起导航" [expanded] [ref=f1e64] [cursor=pointer]
          - generic [ref=f1e68]:
            - heading "组织与成员" [level=1] [ref=f1e69]
            - paragraph [ref=f1e70]: 监测水质、设备与报警状态
        - generic [ref=f1e71]:
          - generic [ref=f1e72]: 演示数据
          - generic [ref=f1e74]: 同源 API · /admin/v1
          - button "管 管理员 ⌄" [ref=f1e76] [cursor=pointer]:
            - generic [ref=f1e77]: 管
            - generic [ref=f1e78]: 管理员
            - generic [ref=f1e79]: ⌄
      - main [ref=f1e80]:
        - generic [ref=f1e81]:
          - generic [ref=f1e83]:
            - heading "组织与成员" [level=2] [ref=f1e84]
            - paragraph [ref=f1e85]: 切换组织并维护成员角色、启停状态和支持授权期限。
          - generic [ref=f1e86]:
            - generic [ref=f1e87]:
              - heading "组织" [level=3] [ref=f1e89]
              - 'button "演示组织 #1 已启用" [ref=f1e91] [cursor=pointer]':
                - generic [ref=f1e92]:
                  - strong [ref=f1e93]: 演示组织
                  - generic [ref=f1e94]: "#1"
                - generic [ref=f1e95]: 已启用
            - generic [ref=f1e97]:
              - generic [ref=f1e99]:
                - heading "演示组织" [level=3] [ref=f1e100]
                - text: 角色变更会使旧权限版本失效
              - generic [ref=f1e103]:
                - table [ref=f1e105]:
                  - rowgroup [ref=f1e112]:
                    - row [ref=f1e113]:
                      - columnheader "成员" [ref=f1e114]
                      - columnheader "角色" [ref=f1e116]
                      - columnheader "支持到期（上海时间）" [ref=f1e118]
                      - columnheader "状态" [ref=f1e120]
                      - columnheader "操作" [ref=f1e122]
                - generic [ref=f1e125]:
                  - table [ref=f1e128]:
                    - rowgroup [ref=f1e135]:
                      - row [ref=f1e136]:
                        - cell "秒数测试成员" [ref=f1e137]
                        - cell "support" [ref=f1e139]:
                          - generic [ref=f1e142] [cursor=pointer]:
                            - generic:
                              - combobox [ref=f1e144]
                              - generic [ref=f1e145]: support
                        - cell [ref=f1e150]:
                          - combobox "支持角色必填" [ref=f1e160]: 2099-10-02 13:00
                        - cell [ref=f1e161]:
                          - generic [ref=f1e163]:
                            - switch [checked]
                            - generic [ref=f1e164] [cursor=pointer]
                        - cell [ref=f1e166]:
                          - button "保存" [active] [ref=f1e168] [cursor=pointer]
                  - generic [ref=f1e171] [cursor=pointer]
  - alert [ref=f1e172]:
    - paragraph [ref=f1e176]: 成员权限已更新
```

# Test source

```ts
  1  | import { expect, test } from '@playwright/test'
  2  | 
  3  | for (const timezoneId of ['Asia/Shanghai', 'UTC', 'America/New_York']) {
  4  |   test.describe(`支持到期：${timezoneId}`, () => {
  5  |     test.use({ timezoneId })
  6  |     test('未编辑到期时间的重复保存保留 UTC 秒数', async ({ page }, testInfo) => {
  7  |       await page.addInitScript(() => {
  8  |         if (!localStorage.getItem('iolink.demo.tenant-members')) {
  9  |           localStorage.setItem('iolink.demo.tenant-members', JSON.stringify([
  10 |             { tenantId: 1, userId: 2, name: '秒数测试成员', role: 'support', active: true, expiresAt: '2099-10-02T05:00:37Z' },
  11 |           ]))
  12 |         }
  13 |       })
  14 |       await page.goto('/login')
  15 |       await page.getByRole('button', { name: '进入控制台' }).click()
  16 |       await expect(page).toHaveURL(/\/dashboard$/)
  17 |       await page.goto('/tenants')
  18 |       const member = page.getByRole('row', { name: /秒数测试成员/ })
  19 |       await expect(member).toBeVisible()
  20 |       for (let save = 1; save <= 2; save++) {
  21 |         await member.getByRole('button', { name: '保存' }).click()
  22 |         await expect(page.getByText('成员权限已更新')).toBeVisible()
  23 |         const persisted = await page.evaluate(() => JSON.parse(localStorage.getItem('iolink.demo.tenant-members') ?? 'null'))
  24 |         await testInfo.attach(`unchanged-save-${save}`, { body: JSON.stringify(persisted, null, 2), contentType: 'application/json' })
> 25 |         expect(persisted).toEqual([
     |                           ^ Error: expect(received).toEqual(expected) // deep equality
  26 |           { tenantId: 1, userId: 2, name: '秒数测试成员', role: 'support', active: true, expiresAt: '2099-10-02T05:00:37Z' },
  27 |         ])
  28 |         await page.reload()
  29 |         await expect(member.locator('input[placeholder="支持角色必填"]')).toHaveValue('2099-10-02 13:00:37')
  30 |       }
  31 |       for (const width of [375, 768, 1280]) {
  32 |         await page.setViewportSize({ width, height: 960 })
  33 |         await page.screenshot({ path: testInfo.outputPath(`unchanged-${width}.png`), fullPage: true })
  34 |       }
  35 |     })
  36 | 
  37 |     test('上海时间显示、编辑、保存和重载保持同一瞬间', async ({ page }, testInfo) => {
  38 |       await page.addInitScript(() => {
  39 |         if (!localStorage.getItem('iolink.demo.tenant-members')) {
  40 |           localStorage.setItem('iolink.demo.tenant-members', JSON.stringify([
  41 |             { tenantId: 1, userId: 2, name: '时区测试成员', role: 'support', active: true, expiresAt: '2099-10-02T05:00:37Z' },
  42 |           ]))
  43 |         }
  44 |       })
  45 |       await page.goto('/login')
  46 |       await page.getByRole('button', { name: '进入控制台' }).click()
  47 |       await expect(page).toHaveURL(/\/dashboard$/)
  48 |       await page.goto('/tenants')
  49 |       const member = page.getByRole('row', { name: /时区测试成员/ })
  50 |       await expect(member).toBeVisible()
  51 |       const expiry = member.locator('input[placeholder="支持角色必填"]')
  52 |       await expect(expiry).toHaveValue('2099-10-02 13:00:37')
  53 |       await expiry.fill('2099-10-03 13:00:47')
  54 |       await expiry.press('Enter')
  55 |       await expiry.blur()
  56 |       await expect(expiry).toHaveValue('2099-10-03 13:00:47')
  57 |       await member.getByRole('button', { name: '保存' }).click()
  58 |       await expect(page.getByText('成员权限已更新')).toBeVisible()
  59 |       const persisted = await page.evaluate(() => JSON.parse(localStorage.getItem('iolink.demo.tenant-members') ?? 'null'))
  60 |       await testInfo.attach('edited-save', { body: JSON.stringify(persisted, null, 2), contentType: 'application/json' })
  61 |       expect(persisted).toEqual([
  62 |         { tenantId: 1, userId: 2, name: '时区测试成员', role: 'support', active: true, expiresAt: '2099-10-03T05:00:47Z' },
  63 |       ])
  64 |       await page.reload()
  65 |       await expect(page.getByRole('row', { name: /时区测试成员/ }).locator('input').nth(1)).toHaveValue('2099-10-03 13:00:47')
  66 |       await page.screenshot({ path: testInfo.outputPath('edited-reloaded.png'), fullPage: true })
  67 |     })
  68 |   })
  69 | }
  70 | 
```