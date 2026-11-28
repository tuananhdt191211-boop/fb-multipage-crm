// Nội dung Chính sách quyền riêng tư, Điều khoản dịch vụ, Hướng dẫn xoá dữ liệu.
// Viết theo dữ liệu ứng dụng thực sự thu thập (xem backend-go/internal/models, handlers).
// Khi thêm tính năng mới có thu thập dữ liệu khác, cập nhật lại file này và ngày LEGAL_UPDATED.

export type LegalLang = 'vi' | 'en'
export type LegalDoc = 'privacy' | 'terms' | 'deletion'

export interface LegalSection {
  h: string
  p?: string[]
  list?: string[]
}

export interface LegalPage {
  title: string
  intro: string[]
  sections: LegalSection[]
}

export const APP_NAME = 'PagePilot'
export const LEGAL_UPDATED = '2026-11-28'

// Email liên hệ công khai: đặt VITE_CONTACT_EMAIL khi build (bắt buộc trước khi nộp Meta App Review)
export const CONTACT_EMAIL = String(import.meta.env.VITE_CONTACT_EMAIL || '').trim()

const contactVi = CONTACT_EMAIL ? `email ${CONTACT_EMAIL}` : 'email hỗ trợ ghi ở cuối trang'
const contactEn = CONTACT_EMAIL ? `email ${CONTACT_EMAIL}` : 'the support email listed at the bottom of this page'

const vi: Record<LegalDoc, LegalPage> = {
  privacy: {
    title: 'Chính sách quyền riêng tư',
    intro: [
      `${APP_NAME} là phần mềm giúp chủ shop quản lý nhiều Fanpage Facebook, hộp thư Messenger, đăng bài, chăm sóc khách hàng và tạo website bán hàng. Chính sách này giải thích chúng tôi thu thập dữ liệu gì, dùng để làm gì, chia sẻ với ai và bạn có những quyền gì.`,
      `Khi sử dụng ${APP_NAME}, bạn đồng ý với chính sách này. Nếu không đồng ý, vui lòng ngừng sử dụng dịch vụ.`
    ],
    sections: [
      {
        h: '1. Dữ liệu chúng tôi thu thập',
        list: [
          'Thông tin tài khoản: họ tên, email, ảnh đại diện, mật khẩu (được mã hoá một chiều, chúng tôi không đọc được). Nếu đăng nhập bằng Google hoặc Facebook, chúng tôi nhận họ tên, email, ảnh đại diện và mã định danh tài khoản do nền tảng đó cung cấp.',
          'Dữ liệu Fanpage bạn kết nối: tên, mã và ảnh của Page, danh mục Page, mã truy cập Page (Page Access Token, được mã hoá trước khi lưu).',
          'Dữ liệu hộp thư Messenger của Page: danh sách hội thoại, tin nhắn, tệp đính kèm, tên và ảnh đại diện công khai của người nhắn tin, mã định danh người nhắn (PSID) do Facebook cấp cho Page.',
          'Dữ liệu chăm sóc khách hàng do bạn hoặc nhân viên nhập: số điện thoại, địa chỉ, ghi chú, trạng thái khách, lịch hẹn chăm sóc.',
          'Nội dung bạn tạo: bài viết, bài hẹn giờ đăng, mẫu tin nhắn, chiến dịch gửi tin, ảnh và video tải lên, cài đặt AI.',
          'Website bán hàng: giao diện, sản phẩm, đơn hàng; thông tin khách mua hàng trên website (họ tên, số điện thoại, email, địa chỉ giao hàng, ghi chú, mật khẩu tài khoản khách được mã hoá một chiều); email đăng ký nhận tin.',
          'Thống kê website bán hàng: trang đã xem, nguồn truy cập, loại thiết bị, quốc gia, các bước trong giỏ hàng và thanh toán. Thống kê này dùng mã ẩn danh của trình duyệt, không lưu địa chỉ IP, họ tên hay số điện thoại.',
          'Khoá kết nối cổng thanh toán của shop (VNPay, PayPal, Stripe) được mã hoá trước khi lưu. Chúng tôi không nhận và không lưu số thẻ ngân hàng của người mua.',
          'Nhật ký kỹ thuật cần thiết để vận hành và bảo mật hệ thống (thời điểm đăng nhập, lỗi hệ thống).'
        ]
      },
      {
        h: '2. Dữ liệu từ Facebook và Instagram',
        p: [
          'Chúng tôi chỉ truy cập dữ liệu Facebook/Instagram mà bạn cho phép trong hộp thoại cấp quyền của Meta, và chỉ dùng cho các tính năng bạn đang sử dụng: hiển thị và trả lời tin nhắn, đồng bộ hội thoại, gửi tin cho khách đã nhắn tin với Page, đăng và hẹn giờ bài viết, đọc thông tin Page.',
          'Chúng tôi không bán dữ liệu Facebook/Instagram, không dùng dữ liệu đó để quảng cáo cho bên thứ ba, và không chuyển dữ liệu đó cho bên môi giới dữ liệu. Việc sử dụng tuân theo Điều khoản nền tảng của Meta (Meta Platform Terms) và Chính sách dành cho nhà phát triển.',
          'Bạn có thể thu hồi quyền bất cứ lúc nào trong Cài đặt Facebook → Bảo mật và đăng nhập → Ứng dụng và trang web (Business Integrations), hoặc ngắt kết nối Page trong ứng dụng.'
        ]
      },
      {
        h: '3. Mục đích sử dụng dữ liệu',
        list: [
          'Cung cấp các tính năng của dịch vụ: hộp thư gộp, chăm sóc khách hàng, đăng bài, gửi tin, website bán hàng, thống kê.',
          'Xác thực đăng nhập, phân quyền nhân viên và bảo vệ tài khoản.',
          'Xử lý đơn hàng và thanh toán của website bán hàng.',
          'Hỗ trợ kỹ thuật, khắc phục lỗi và cải thiện dịch vụ.',
          'Tuân thủ nghĩa vụ pháp lý.'
        ]
      },
      {
        h: '4. Tính năng AI',
        p: [
          'Khi bạn dùng tính năng AI (viết bài, gợi ý hoặc tự động trả lời tin nhắn, dịch website, viết kịch bản), nội dung cần xử lý, ví dụ đoạn hội thoại hoặc mô tả sản phẩm, được gửi tới nhà cung cấp mô hình AI (hiện là Google Gemini) để tạo câu trả lời. Chúng tôi chỉ gửi phần nội dung cần thiết cho yêu cầu đó.',
          'Bạn có thể tắt tính năng AI tự động trả lời trong phần cài đặt AI của từng Page.'
        ]
      },
      {
        h: '5. Chia sẻ dữ liệu với bên thứ ba',
        p: ['Chúng tôi không bán dữ liệu cá nhân. Dữ liệu chỉ được chia sẻ với các bên cần thiết để vận hành dịch vụ:'],
        list: [
          'Meta (Facebook, Messenger, Instagram): để gửi, nhận tin nhắn và đăng bài theo yêu cầu của bạn.',
          'Google: đăng nhập bằng Google và mô hình AI Gemini.',
          'Cổng thanh toán VNPay, PayPal, Stripe: khi người mua chọn thanh toán qua cổng đó.',
          'Nhà cung cấp hạ tầng: máy chủ, cơ sở dữ liệu, lưu trữ và phân phối website (ví dụ Render, Cloudflare và nhà cung cấp cơ sở dữ liệu PostgreSQL).',
          'Cơ quan nhà nước có thẩm quyền khi pháp luật yêu cầu.'
        ]
      },
      {
        h: '6. Lưu trữ và bảo mật',
        list: [
          'Mã truy cập Page và khoá cổng thanh toán được mã hoá trước khi lưu; mật khẩu được băm một chiều.',
          'Kết nối tới dịch vụ dùng HTTPS.',
          'Dữ liệu được lưu trong thời gian tài khoản còn hoạt động. Khi bạn yêu cầu xoá, chúng tôi xoá trong tối đa 30 ngày, trừ dữ liệu pháp luật bắt buộc lưu giữ.',
          'Dữ liệu thống kê website cũ được tự động dọn định kỳ.'
        ]
      },
      {
        h: '7. Quyền của bạn',
        list: [
          'Xem, sửa thông tin tài khoản và dữ liệu khách hàng trong ứng dụng.',
          'Ngắt kết nối Fanpage và thu hồi quyền Facebook bất cứ lúc nào.',
          'Yêu cầu xuất hoặc xoá dữ liệu: xem trang Hướng dẫn xoá dữ liệu.',
          `Khiếu nại hoặc hỏi về quyền riêng tư: liên hệ ${contactVi}.`
        ]
      },
      {
        h: '8. Trách nhiệm của chủ shop',
        p: [
          `Chủ shop sử dụng ${APP_NAME} là bên kiểm soát dữ liệu khách hàng của mình (người nhắn tin với Page, người mua trên website). Chủ shop chịu trách nhiệm thông báo cho khách hàng và có cơ sở hợp pháp khi thu thập, sử dụng dữ liệu đó, bao gồm tuân thủ chính sách nhắn tin của Meta và pháp luật về bảo vệ dữ liệu cá nhân (tại Việt Nam là Nghị định 13/2023/NĐ-CP).`
        ]
      },
      {
        h: '9. Trẻ em',
        p: ['Dịch vụ dành cho người từ 18 tuổi trở lên dùng cho mục đích kinh doanh. Chúng tôi không cố ý thu thập dữ liệu của trẻ em.']
      },
      {
        h: '10. Thay đổi chính sách',
        p: ['Chúng tôi có thể cập nhật chính sách này. Ngày cập nhật gần nhất ghi ở đầu trang. Thay đổi quan trọng sẽ được thông báo trong ứng dụng.']
      }
    ]
  },
  terms: {
    title: 'Điều khoản dịch vụ',
    intro: [`Điều khoản này quy định việc bạn sử dụng ${APP_NAME}. Khi tạo tài khoản hoặc sử dụng dịch vụ, bạn đồng ý với các điều khoản dưới đây.`],
    sections: [
      {
        h: '1. Dịch vụ',
        p: [`${APP_NAME} cung cấp công cụ quản lý Fanpage, hộp thư Messenger, đăng bài, gửi tin chăm sóc khách hàng, tính năng AI và website bán hàng. Một số tính năng có thể thay đổi, giới hạn theo gói hoặc ngừng cung cấp.`]
      },
      {
        h: '2. Tài khoản',
        list: [
          'Bạn phải từ 18 tuổi trở lên và cung cấp thông tin chính xác.',
          'Bạn chịu trách nhiệm giữ bí mật mật khẩu và mọi hoạt động trong tài khoản, kể cả tài khoản nhân viên bạn tạo.',
          'Bạn chỉ được kết nối những Fanpage mà bạn có quyền quản lý hợp pháp.'
        ]
      },
      {
        h: '3. Sử dụng hợp lệ',
        p: ['Bạn cam kết không dùng dịch vụ để:'],
        list: [
          'Gửi tin rác, tin quảng cáo trái với chính sách nhắn tin của Meta, hoặc nhắn cho người đã từ chối nhận tin.',
          'Lừa đảo, mạo danh, bán hàng cấm, hàng giả hoặc vi phạm quyền sở hữu trí tuệ.',
          'Thu thập dữ liệu người khác trái phép.',
          'Phát tán mã độc, tấn công hoặc làm quá tải hệ thống.',
          'Vi phạm Điều khoản và Chính sách của Meta, Google hoặc pháp luật hiện hành.'
        ]
      },
      {
        h: '4. Nền tảng bên thứ ba',
        p: [
          'Dịch vụ kết nối với Facebook, Messenger, Instagram, Google và các cổng thanh toán. Việc bạn sử dụng các nền tảng này còn chịu điều khoản riêng của họ. Chúng tôi không chịu trách nhiệm khi nền tảng bên thứ ba thay đổi, giới hạn hoặc khoá tính năng, Page hay tài khoản của bạn.',
          'Tính năng gửi tin hàng loạt có giới hạn tốc độ và khung giờ để giảm rủi ro, nhưng bạn vẫn chịu trách nhiệm về nội dung và đối tượng nhận tin.'
        ]
      },
      {
        h: '5. Nội dung và dữ liệu của bạn',
        p: [
          'Bạn giữ quyền sở hữu nội dung và dữ liệu của mình. Bạn cho phép chúng tôi lưu trữ và xử lý dữ liệu đó trong phạm vi cần thiết để cung cấp dịch vụ, theo Chính sách quyền riêng tư.',
          'Nội dung do AI tạo ra chỉ mang tính gợi ý. Bạn có trách nhiệm kiểm tra trước khi đăng hoặc gửi cho khách hàng.'
        ]
      },
      {
        h: '6. Website bán hàng và thanh toán',
        p: ['Chủ shop là người bán và chịu trách nhiệm về sản phẩm, giá, giao hàng, đổi trả, hoá đơn và nghĩa vụ thuế. Tiền thanh toán qua cổng thanh toán được chuyển thẳng vào tài khoản của chủ shop; chúng tôi không giữ tiền của người mua.']
      },
      {
        h: '7. Phí dịch vụ',
        p: ['Các gói trả phí (nếu có) được công bố trong ứng dụng. Phí đã thanh toán không hoàn lại trừ khi pháp luật yêu cầu hoặc có thoả thuận khác.']
      },
      {
        h: '8. Tạm ngưng và chấm dứt',
        p: ['Chúng tôi có thể tạm khoá hoặc chấm dứt tài khoản vi phạm điều khoản này. Bạn có thể ngừng sử dụng và yêu cầu xoá dữ liệu bất cứ lúc nào.']
      },
      {
        h: '9. Giới hạn trách nhiệm',
        p: ['Dịch vụ được cung cấp theo hiện trạng. Trong phạm vi pháp luật cho phép, chúng tôi không chịu trách nhiệm về thiệt hại gián tiếp như mất doanh thu, mất dữ liệu hoặc gián đoạn kinh doanh do lỗi kết nối, nền tảng bên thứ ba hoặc sự kiện ngoài tầm kiểm soát.']
      },
      {
        h: '10. Luật áp dụng',
        p: ['Điều khoản này tuân theo pháp luật Việt Nam. Tranh chấp được ưu tiên giải quyết bằng thương lượng.']
      },
      {
        h: '11. Liên hệ',
        p: [`Mọi câu hỏi về điều khoản, vui lòng liên hệ ${contactVi}.`]
      }
    ]
  },
  deletion: {
    title: 'Hướng dẫn xoá dữ liệu',
    intro: [`Bạn có thể yêu cầu xoá dữ liệu của mình trên ${APP_NAME} bất cứ lúc nào bằng một trong các cách dưới đây.`],
    sections: [
      {
        h: 'Cách 1: Gỡ ứng dụng trong cài đặt Facebook',
        list: [
          'Mở Facebook, vào Cài đặt và quyền riêng tư → Cài đặt.',
          'Chọn Bảo mật và đăng nhập → Ứng dụng và trang web (hoặc Business Integrations).',
          `Tìm ${APP_NAME}, bấm Gỡ và chọn gửi yêu cầu xoá dữ liệu.`,
          'Hệ thống tự động xoá mã truy cập Fanpage, ngắt kết nối các Page, xoá liên kết tài khoản Facebook và ảnh đại diện. Facebook sẽ hiển thị một mã xác nhận; bạn có thể nhập mã đó ở ô bên dưới để xem trạng thái.'
        ]
      },
      {
        h: 'Cách 2: Ngắt kết nối trong ứng dụng',
        list: [
          `Đăng nhập ${APP_NAME}, vào mục Quản lý Fanpage và ngắt kết nối Page cần xoá.`,
          'Mã truy cập của Page sẽ không còn được dùng để đọc hay gửi tin nhắn.'
        ]
      },
      {
        h: 'Cách 3: Xoá toàn bộ tài khoản',
        list: [
          `Gửi yêu cầu tới ${contactVi} từ email đăng ký tài khoản, tiêu đề "Yêu cầu xoá dữ liệu".`,
          'Chúng tôi xác minh và xoá toàn bộ dữ liệu tài khoản, gồm hội thoại, khách hàng, bài viết, website, sản phẩm và đơn hàng, trong tối đa 30 ngày, rồi gửi email xác nhận.',
          'Một số dữ liệu có thể được giữ lại nếu pháp luật bắt buộc (ví dụ chứng từ giao dịch).'
        ]
      }
    ]
  }
}

const en: Record<LegalDoc, LegalPage> = {
  privacy: {
    title: 'Privacy Policy',
    intro: [
      `${APP_NAME} is software that helps sellers manage multiple Facebook Pages, Messenger inboxes, posting, customer care and online stores. This policy explains what data we collect, how we use it, who we share it with and what rights you have.`,
      `By using ${APP_NAME} you agree to this policy. If you do not agree, please stop using the service.`
    ],
    sections: [
      {
        h: '1. Data we collect',
        list: [
          'Account information: name, email, profile picture and password (stored as a one-way hash we cannot read). If you sign in with Google or Facebook, we receive your name, email, profile picture and the account identifier provided by that platform.',
          'Facebook Pages you connect: Page name, ID, picture, category and the Page Access Token (encrypted before storage).',
          'Page Messenger inbox data: conversations, messages, attachments, the public name and profile picture of people who message the Page, and the Page-scoped ID (PSID) assigned by Facebook.',
          'Customer-care data entered by you or your staff: phone number, address, notes, customer status and follow-up reminders.',
          'Content you create: posts, scheduled posts, message templates, messaging campaigns, uploaded images and videos, AI settings.',
          'Online store: design, products, orders; information of buyers on the store (name, phone, email, shipping address, notes, buyer account password stored as a one-way hash); newsletter emails.',
          'Store analytics: pages viewed, traffic source, device type, country and cart/checkout steps, recorded with an anonymous browser ID. We do not store IP addresses, names or phone numbers in analytics.',
          'Shop payment gateway keys (VNPay, PayPal, Stripe) are encrypted before storage. We never receive or store buyers\' card numbers.',
          'Technical logs needed to operate and secure the system (sign-in times, errors).'
        ]
      },
      {
        h: '2. Data from Facebook and Instagram',
        p: [
          'We only access the Facebook/Instagram data you authorize in Meta\'s permission dialog, and only use it for the features you use: showing and replying to messages, syncing conversations, messaging people who have contacted your Page, publishing and scheduling posts, and reading Page information.',
          'We do not sell Facebook/Instagram data, do not use it to advertise for third parties and do not transfer it to data brokers. Our use complies with the Meta Platform Terms and Developer Policies.',
          'You can revoke access at any time in Facebook Settings → Security and login → Apps and websites (Business Integrations), or by disconnecting a Page in the app.'
        ]
      },
      {
        h: '3. How we use data',
        list: [
          'To provide the service: unified inbox, customer care, posting, messaging, online store and analytics.',
          'To authenticate users, manage staff permissions and protect accounts.',
          'To process store orders and payments.',
          'To provide support, fix problems and improve the service.',
          'To comply with legal obligations.'
        ]
      },
      {
        h: '4. AI features',
        p: [
          'When you use AI features (writing posts, suggested or automatic replies, store translation, scripts), the content needed for that request, such as a conversation excerpt or product description, is sent to our AI model provider (currently Google Gemini) to generate a response. We only send the content required for that request.',
          'You can turn off AI auto-reply in each Page\'s AI settings.'
        ]
      },
      {
        h: '5. Sharing with third parties',
        p: ['We do not sell personal data. Data is shared only with parties needed to run the service:'],
        list: [
          'Meta (Facebook, Messenger, Instagram): to send and receive messages and publish posts at your request.',
          'Google: Google sign-in and the Gemini AI model.',
          'Payment gateways VNPay, PayPal, Stripe: when a buyer chooses to pay through them.',
          'Infrastructure providers: servers, database, storage and website delivery (for example Render, Cloudflare and our PostgreSQL database provider).',
          'Competent authorities when required by law.'
        ]
      },
      {
        h: '6. Storage and security',
        list: [
          'Page Access Tokens and payment keys are encrypted before storage; passwords are one-way hashed.',
          'Connections to the service use HTTPS.',
          'Data is kept while your account is active. When you request deletion we delete it within 30 days, except data we must keep by law.',
          'Old store analytics data is cleaned up automatically.'
        ]
      },
      {
        h: '7. Your rights',
        list: [
          'View and edit your account and customer data in the app.',
          'Disconnect Pages and revoke Facebook permissions at any time.',
          'Request export or deletion of your data: see the Data Deletion Instructions page.',
          `Questions or complaints about privacy: contact ${contactEn}.`
        ]
      },
      {
        h: '8. Responsibilities of sellers',
        p: [
          `Sellers using ${APP_NAME} are the controllers of their own customers' data (people messaging their Page, buyers on their store). Sellers are responsible for informing their customers and having a lawful basis to collect and use that data, including complying with Meta's messaging policies and applicable data-protection laws (in Vietnam, Decree 13/2023/ND-CP).`
        ]
      },
      {
        h: '9. Children',
        p: ['The service is intended for people aged 18 or older for business use. We do not knowingly collect children\'s data.']
      },
      {
        h: '10. Changes',
        p: ['We may update this policy. The date of the latest update is shown at the top of this page. Important changes will be announced in the app.']
      }
    ]
  },
  terms: {
    title: 'Terms of Service',
    intro: [`These terms govern your use of ${APP_NAME}. By creating an account or using the service you agree to them.`],
    sections: [
      {
        h: '1. The service',
        p: [`${APP_NAME} provides tools for managing Facebook Pages, Messenger inboxes, posting, customer-care messaging, AI features and online stores. Features may change, be limited by plan or be discontinued.`]
      },
      {
        h: '2. Accounts',
        list: [
          'You must be 18 or older and provide accurate information.',
          'You are responsible for keeping your password secret and for all activity in your account, including staff accounts you create.',
          'You may only connect Pages you are lawfully authorized to manage.'
        ]
      },
      {
        h: '3. Acceptable use',
        p: ['You agree not to use the service to:'],
        list: [
          'Send spam or promotional messages that violate Meta\'s messaging policies, or message people who have opted out.',
          'Commit fraud, impersonate others, sell prohibited or counterfeit goods, or infringe intellectual property.',
          'Collect other people\'s data unlawfully.',
          'Spread malware, attack or overload the system.',
          'Violate the terms and policies of Meta, Google or applicable law.'
        ]
      },
      {
        h: '4. Third-party platforms',
        p: [
          'The service connects to Facebook, Messenger, Instagram, Google and payment gateways. Your use of those platforms is also subject to their own terms. We are not responsible if a third-party platform changes, limits or blocks features, Pages or accounts.',
          'Bulk messaging includes rate and time-window limits to reduce risk, but you remain responsible for message content and recipients.'
        ]
      },
      {
        h: '5. Your content and data',
        p: [
          'You keep ownership of your content and data. You allow us to store and process it as needed to provide the service, as described in the Privacy Policy.',
          'AI-generated content is only a suggestion. You are responsible for reviewing it before publishing or sending it to customers.'
        ]
      },
      {
        h: '6. Online stores and payments',
        p: ['The seller is responsible for products, prices, delivery, returns, invoices and taxes. Payments made through payment gateways go directly to the seller\'s account; we do not hold buyers\' money.']
      },
      {
        h: '7. Fees',
        p: ['Paid plans, if any, are shown in the app. Fees paid are non-refundable unless required by law or otherwise agreed.']
      },
      {
        h: '8. Suspension and termination',
        p: ['We may suspend or terminate accounts that violate these terms. You may stop using the service and request deletion of your data at any time.']
      },
      {
        h: '9. Limitation of liability',
        p: ['The service is provided as is. To the extent permitted by law, we are not liable for indirect damages such as lost revenue, lost data or business interruption caused by connection problems, third-party platforms or events beyond our control.']
      },
      {
        h: '10. Governing law',
        p: ['These terms are governed by the laws of Vietnam. Disputes will first be resolved through negotiation.']
      },
      {
        h: '11. Contact',
        p: [`For questions about these terms, contact ${contactEn}.`]
      }
    ]
  },
  deletion: {
    title: 'Data Deletion Instructions',
    intro: [`You can request deletion of your data on ${APP_NAME} at any time using one of the options below.`],
    sections: [
      {
        h: 'Option 1: Remove the app in Facebook settings',
        list: [
          'Open Facebook and go to Settings & privacy → Settings.',
          'Select Security and login → Apps and websites (or Business Integrations).',
          `Find ${APP_NAME}, click Remove and choose to send a data deletion request.`,
          'We automatically delete your Page Access Tokens, disconnect your Pages and remove your Facebook account link and profile picture. Facebook will show a confirmation code; enter it in the box below to check the status.'
        ]
      },
      {
        h: 'Option 2: Disconnect in the app',
        list: [
          `Sign in to ${APP_NAME}, open Page management and disconnect the Page.`,
          'The Page Access Token will no longer be used to read or send messages.'
        ]
      },
      {
        h: 'Option 3: Delete your whole account',
        list: [
          `Send a request to ${contactEn} from your account email with the subject "Data deletion request".`,
          'We verify the request and delete all account data, including conversations, customers, posts, websites, products and orders, within 30 days, then email you a confirmation.',
          'Some data may be retained where required by law (for example transaction records).'
        ]
      }
    ]
  }
}

export const legalContent: Record<LegalLang, Record<LegalDoc, LegalPage>> = { vi, en }
