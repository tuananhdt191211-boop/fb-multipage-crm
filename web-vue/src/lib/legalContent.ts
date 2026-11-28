// Nội dung 3 trang pháp lý CÔNG KHAI của chính ứng dụng PagePilot (Meta bắt buộc khi dùng Facebook Login / Messenger API):
//   /privacy       Chính sách quyền riêng tư (Privacy Policy URL)
//   /terms         Điều khoản dịch vụ (Terms of Service URL)
//   /data-deletion Hướng dẫn xoá dữ liệu (User Data Deletion → Data deletion instructions URL)
// Song ngữ Việt / Anh (người duyệt của Meta đọc tiếng Anh). Không nằm trong danh mục dịch giao diện quản trị.
// Tên đơn vị vận hành và email liên hệ đặt lúc build qua VITE_OPERATOR_NAME / VITE_SUPPORT_EMAIL (không ghi vào mã nguồn).
// LƯU Ý: đây là bản soạn sẵn theo đúng cách ứng dụng đang hoạt động — chủ ứng dụng cần đọc lại và chỉnh theo thực tế vận hành.

export type LegalKey = 'privacy' | 'terms' | 'deletion'
export type LegalLang = 'vi' | 'en'

export interface LegalSection {
  h: string
  p?: string[]
  ul?: string[]
  after?: string[]
}

export interface LegalDoc {
  title: string
  summary: string
  sections: LegalSection[]
}

export const APP_NAME = 'PagePilot'
export const LEGAL_UPDATED: Record<LegalLang, string> = { vi: '28/11/2026', en: 'November 28, 2026' }

export const contact = {
  email: String(import.meta.env.VITE_SUPPORT_EMAIL || '').trim(),
  operator: String(import.meta.env.VITE_OPERATOR_NAME || '').trim()
}

export const legalPaths: Record<LegalKey, string> = { privacy: '/privacy', terms: '/terms', deletion: '/data-deletion' }

export const legalTitles: Record<LegalKey, Record<LegalLang, string>> = {
  privacy: { vi: 'Chính sách quyền riêng tư', en: 'Privacy Policy' },
  terms: { vi: 'Điều khoản dịch vụ', en: 'Terms of Service' },
  deletion: { vi: 'Hướng dẫn xoá dữ liệu', en: 'Data Deletion Instructions' }
}

const who = (lang: LegalLang) =>
  contact.operator ? (lang === 'vi' ? `${APP_NAME} do ${contact.operator} vận hành` : `${APP_NAME} is operated by ${contact.operator}`) : lang === 'vi' ? `${APP_NAME} do đơn vị vận hành ứng dụng cung cấp` : `${APP_NAME} is provided by the operator of the application`

const contactSection = (lang: LegalLang): LegalSection =>
  lang === 'vi'
    ? {
        h: 'Liên hệ',
        p: contact.email
          ? [`Mọi câu hỏi về quyền riêng tư, điều khoản hoặc yêu cầu xoá dữ liệu, vui lòng gửi email tới ${contact.email}.`]
          : ['Mọi câu hỏi về quyền riêng tư, điều khoản hoặc yêu cầu xoá dữ liệu, vui lòng liên hệ đơn vị vận hành ứng dụng qua kênh hỗ trợ đã cung cấp cho bạn khi đăng ký.']
      }
    : {
        h: 'Contact',
        p: contact.email
          ? [`For any question about privacy, these terms or a data deletion request, email ${contact.email}.`]
          : ['For any question about privacy, these terms or a data deletion request, contact the operator of the application through the support channel provided to you when you signed up.']
      }

// ----------------------------------------------------------------------------------------------- Quyền riêng tư
function privacy(lang: LegalLang): LegalDoc {
  if (lang === 'vi')
    return {
      title: 'Chính sách quyền riêng tư',
      summary: `Chính sách này giải thích ${APP_NAME} thu thập, sử dụng, chia sẻ và bảo vệ dữ liệu của bạn như thế nào, và bạn có thể làm gì với dữ liệu đó.`,
      sections: [
        { h: '1. Chúng tôi là ai', p: [`${APP_NAME} (CRM đa Fanpage) là ứng dụng web giúp chủ Fanpage Facebook quản lý hộp thư gộp, trả lời khách hàng (kể cả bằng AI), lên lịch đăng bài, theo dõi đối thủ, chăm sóc khách cũ và dựng website bán hàng. ${who('vi')}.`] },
        {
          h: '2. Dữ liệu chúng tôi thu thập',
          p: ['Tuỳ chức năng bạn dùng, chúng tôi xử lý các nhóm dữ liệu sau:'],
          ul: [
            'Tài khoản: họ tên, email, ảnh đại diện, cách đăng nhập (email và mật khẩu — mật khẩu chỉ lưu dạng băm, không lưu bản gốc —, Google hoặc Facebook).',
            'Từ Facebook, khi bạn cho phép: mã người dùng, tên, email, ảnh đại diện; danh sách Fanpage bạn quản lý (tên, mã, danh mục, ảnh); mã truy cập Fanpage (Page Access Token, lưu dạng mã hoá AES-256-GCM).',
            'Tin nhắn Fanpage (Messenger): nội dung tin nhắn và tệp đính kèm, mã khách theo phạm vi Fanpage (PSID), tên và ảnh đại diện công khai của người nhắn tin, thời gian gửi. Ghi chú CRM do nhân viên của bạn nhập (số điện thoại, địa chỉ, tình trạng khách, lịch chăm sóc).',
            'Bài đăng Fanpage: nội dung, ảnh, lượt thích / bình luận / chia sẻ — để hiển thị thống kê và lên lịch đăng.',
            'Nội dung bạn tạo trong ứng dụng: bài đăng đã lên lịch, mẫu tin nhắn, cài đặt trợ lý AI, chiến dịch chăm sóc khách, danh sách đối thủ bạn theo dõi (chỉ dữ liệu công khai của họ), website, sản phẩm, đơn hàng.',
            'Khách mua hàng trên website do bạn tạo: họ tên, số điện thoại, email, địa chỉ nhận hàng và nội dung đơn. Thống kê truy cập website ở dạng ẩn danh: không lưu địa chỉ IP, tên, số điện thoại, email hay địa chỉ; tôn trọng tín hiệu Do Not Track.',
            'Dữ liệu kỹ thuật: nhật ký máy chủ, loại trình duyệt, và mã đăng nhập lưu trong trình duyệt (localStorage / cookie) để giữ phiên đăng nhập.'
          ]
        },
        {
          h: '3. Quyền Facebook mà ứng dụng yêu cầu',
          p: ['Khi bạn đăng nhập hoặc kết nối Fanpage bằng Facebook, ứng dụng xin các quyền sau và chỉ dùng cho đúng mục đích nêu:'],
          ul: [
            'public_profile, email — tạo và nhận diện tài khoản của bạn.',
            'pages_show_list — hiển thị danh sách Fanpage bạn quản lý để bạn chọn kết nối.',
            'pages_messaging — nhận và gửi tin nhắn Messenger của Fanpage trong hộp thư gộp.',
            'pages_read_engagement — đọc bài đăng, bình luận, lượt tương tác của Fanpage để thống kê.',
            'pages_manage_metadata — đăng ký nhận sự kiện (webhook) khi Fanpage có tin nhắn mới.'
          ],
          after: ['Bạn có thể thu hồi các quyền này bất cứ lúc nào (xem mục 8 và trang Hướng dẫn xoá dữ liệu). Chúng tôi không dùng dữ liệu từ Facebook cho quảng cáo, không bán, không cho thuê và không chuyển cho môi giới dữ liệu.']
        },
        {
          h: '4. Chúng tôi dùng dữ liệu để làm gì',
          ul: [
            'Cung cấp các chức năng bạn yêu cầu: hộp thư gộp, trả lời khách, đăng bài, chăm sóc khách, thống kê, website bán hàng.',
            'Tạo gợi ý hoặc câu trả lời bằng AI cho tin nhắn của khách và dịch nội dung website (xem mục 5).',
            'Bảo mật, chống lạm dụng, khắc phục lỗi và cải thiện ứng dụng.',
            'Liên lạc với bạn về tài khoản và dịch vụ.'
          ]
        },
        {
          h: '5. Chia sẻ dữ liệu',
          p: ['Chúng tôi không bán dữ liệu cá nhân. Dữ liệu chỉ được chuyển cho bên thứ ba ở mức cần thiết để vận hành dịch vụ:'],
          ul: [
            'Nhà cung cấp hạ tầng: lưu trữ máy chủ và cơ sở dữ liệu, mạng phân phối nội dung, kho lưu ảnh / video (ví dụ Render, Cloudflare, Neon).',
            'Dịch vụ AI: khi bạn bật trợ lý AI hoặc dịch website, nội dung tin nhắn / văn bản liên quan được gửi tới nhà cung cấp mô hình AI (hiện là Google Gemini API hoặc dịch vụ tương thích OpenAI do đơn vị vận hành cấu hình) để tạo kết quả.',
            'Cổng thanh toán (VNPay, PayPal, Stripe) khi website của bạn bật thanh toán trực tuyến — chúng tôi không lưu số thẻ.',
            'Meta (Facebook / Messenger) và Google: khi bạn đăng nhập hoặc gửi / nhận tin nhắn qua nền tảng của họ.',
            'Cơ quan nhà nước có thẩm quyền khi pháp luật yêu cầu.'
          ]
        },
        {
          h: '6. Thời gian lưu trữ',
          ul: [
            'Dữ liệu tài khoản, hội thoại, ghi chú CRM và nội dung bạn tạo được giữ trong thời gian tài khoản còn hoạt động, cho tới khi bạn yêu cầu xoá.',
            'Khi bạn gỡ một Fanpage khỏi ứng dụng, mã truy cập Fanpage đã lưu bị xoá ngay.',
            'Sự kiện thống kê truy cập website tự xoá sau 60 ngày; phiên truy cập tự xoá sau 180 ngày.',
            'Khi bạn yêu cầu xoá tài khoản, chúng tôi xoá dữ liệu trong vòng 30 ngày (trừ dữ liệu pháp luật bắt buộc phải giữ).'
          ]
        },
        {
          h: '7. Bảo mật',
          p: ['Kết nối tới ứng dụng dùng HTTPS. Mật khẩu được băm; mã truy cập Fanpage được mã hoá AES-256-GCM; dữ liệu của mỗi khách hàng được tách riêng theo tài khoản và nhân viên chỉ thấy phần được cấp quyền. Không hệ thống nào an toàn tuyệt đối — nếu phát hiện sự cố ảnh hưởng tới dữ liệu của bạn, chúng tôi sẽ thông báo theo quy định pháp luật.']
        },
        {
          h: '8. Quyền của bạn',
          ul: [
            'Truy cập, chỉnh sửa dữ liệu của bạn ngay trong ứng dụng.',
            'Gỡ kết nối Fanpage (mục Fanpage → "Gỡ Page") hoặc thu hồi quyền của ứng dụng trong cài đặt Facebook.',
            'Yêu cầu xuất hoặc xoá dữ liệu — xem trang Hướng dẫn xoá dữ liệu.',
            'Rút lại sự đồng ý và khiếu nại với cơ quan bảo vệ dữ liệu có thẩm quyền.'
          ]
        },
        {
          h: '9. Người nhắn tin cho Fanpage',
          p: ['Nếu bạn là người đã nhắn tin cho một Fanpage dùng PagePilot, chủ Fanpage là bên quyết định cách xử lý tin nhắn của bạn; PagePilot xử lý thay họ. Bạn có thể yêu cầu xoá cuộc trò chuyện của mình bằng cách liên hệ chủ Fanpage hoặc chúng tôi (xem trang Hướng dẫn xoá dữ liệu).']
        },
        { h: '10. Trẻ em', p: [`${APP_NAME} không dành cho người dưới 16 tuổi và chúng tôi không cố ý thu thập dữ liệu của họ.`] },
        { h: '11. Chuyển dữ liệu ra nước ngoài', p: ['Nhà cung cấp hạ tầng và AI có thể đặt máy chủ ngoài Việt Nam; chúng tôi chỉ chuyển dữ liệu ở mức cần thiết và theo các cam kết bảo vệ dữ liệu của họ.'] },
        { h: '12. Thay đổi chính sách', p: ['Khi có thay đổi quan trọng, chúng tôi sẽ cập nhật trang này và ngày "Cập nhật lần cuối" ở trên. Tiếp tục sử dụng dịch vụ sau khi thay đổi có hiệu lực nghĩa là bạn chấp nhận chính sách mới.'] },
        contactSection('vi')
      ]
    }
  return {
    title: 'Privacy Policy',
    summary: `This policy explains how ${APP_NAME} collects, uses, shares and protects your data, and what you can do with it.`,
    sections: [
      { h: '1. Who we are', p: [`${APP_NAME} (a multi-Page CRM) is a web application that helps Facebook Page owners manage a unified inbox, reply to customers (including with AI), schedule posts, track competitors, follow up with past customers and build online stores. ${who('en')}.`] },
      {
        h: '2. Data we collect',
        p: ['Depending on the features you use, we process the following data:'],
        ul: [
          'Account: full name, email, profile picture and sign-in method (email and password — passwords are stored only as hashes, never in clear text —, Google or Facebook).',
          'From Facebook, when you allow it: your user ID, name, email and profile picture; the list of Pages you manage (name, ID, category, picture); Page access tokens (stored encrypted with AES-256-GCM).',
          'Page messages (Messenger): message text and attachments, the Page-scoped ID (PSID), public name and profile picture of the person who wrote to the Page, and timestamps. CRM notes entered by your staff (phone number, address, lead status, follow-up schedule).',
          'Page posts: content, images, likes / comments / shares — to show statistics and schedule posts.',
          'Content you create in the app: scheduled posts, message templates, AI assistant settings, customer-care campaigns, the competitors you track (their public data only), websites, products and orders.',
          'Buyers on websites you build: full name, phone number, email, delivery address and order contents. Website traffic statistics are anonymous: we do not store IP address, name, phone, email or address, and we respect Do Not Track.',
          'Technical data: server logs, browser type, and the sign-in token kept in your browser (localStorage / cookie) to keep you signed in.'
        ]
      },
      {
        h: '3. Facebook permissions the app requests',
        p: ['When you sign in or connect a Page with Facebook, the app asks for the following permissions and uses them only for the purposes below:'],
        ul: [
          'public_profile, email — to create and recognise your account.',
          'pages_show_list — to list the Pages you manage so you can choose which to connect.',
          'pages_messaging — to receive and send the Page’s Messenger messages in the unified inbox.',
          'pages_read_engagement — to read the Page’s posts, comments and engagement for statistics.',
          'pages_manage_metadata — to subscribe to webhook events when the Page receives new messages.'
        ],
        after: ['You can revoke these permissions at any time (see section 8 and the Data Deletion Instructions page). We do not use Facebook data for advertising, and we do not sell, rent or transfer it to data brokers.']
      },
      {
        h: '4. How we use data',
        ul: [
          'To provide the features you ask for: unified inbox, customer replies, posting, customer care, statistics and online stores.',
          'To generate suggestions or replies with AI for customer messages, and to translate website content (see section 5).',
          'For security, abuse prevention, troubleshooting and improving the application.',
          'To contact you about your account and the service.'
        ]
      },
      {
        h: '5. Sharing',
        p: ['We do not sell personal data. Data is shared with third parties only as needed to run the service:'],
        ul: [
          'Infrastructure providers: server and database hosting, content delivery network, image / video storage (for example Render, Cloudflare, Neon).',
          'AI services: when you turn on the AI assistant or website translation, the relevant message / text content is sent to the AI model provider (currently the Google Gemini API or an OpenAI-compatible service configured by the operator) to produce the result.',
          'Payment gateways (VNPay, PayPal, Stripe) when your store enables online payment — we do not store card numbers.',
          'Meta (Facebook / Messenger) and Google, when you sign in or send / receive messages through their platforms.',
          'Public authorities where required by law.'
        ]
      },
      {
        h: '6. Retention',
        ul: [
          'Account data, conversations, CRM notes and content you create are kept while your account is active, until you ask for deletion.',
          'When you remove a Page from the app, the stored Page access token is deleted immediately.',
          'Website traffic events are deleted automatically after 60 days; visit sessions after 180 days.',
          'When you ask us to delete your account we delete your data within 30 days (except data we are legally required to keep).'
        ]
      },
      {
        h: '7. Security',
        p: ['Connections to the application use HTTPS. Passwords are hashed; Page access tokens are encrypted with AES-256-GCM; each customer’s data is separated by account and staff only see what they are granted. No system is perfectly secure — if an incident affects your data we will notify you as required by law.']
      },
      {
        h: '8. Your rights',
        ul: [
          'Access and edit your data directly in the app.',
          'Disconnect a Page (Pages → "Remove Page") or revoke the app’s permissions in your Facebook settings.',
          'Request export or deletion of your data — see the Data Deletion Instructions page.',
          'Withdraw consent and complain to the competent data protection authority.'
        ]
      },
      {
        h: '9. People who message a Page',
        p: ['If you have sent a message to a Page that uses PagePilot, the Page owner decides how your messages are handled and PagePilot processes them on their behalf. You can ask for your conversation to be deleted by contacting the Page owner or us (see the Data Deletion Instructions page).']
      },
      { h: '10. Children', p: [`${APP_NAME} is not intended for people under 16 and we do not knowingly collect their data.`] },
      { h: '11. International transfers', p: ['Infrastructure and AI providers may operate servers outside Vietnam; we transfer only the data needed and rely on their data protection commitments.'] },
      { h: '12. Changes to this policy', p: ['When we make material changes we will update this page and the “Last updated” date above. Continuing to use the service after a change takes effect means you accept the new policy.'] },
      contactSection('en')
    ]
  }
}

// ----------------------------------------------------------------------------------------------- Điều khoản
function terms(lang: LegalLang): LegalDoc {
  if (lang === 'vi')
    return {
      title: 'Điều khoản dịch vụ',
      summary: `Bằng việc tạo tài khoản hoặc sử dụng ${APP_NAME}, bạn đồng ý với các điều khoản dưới đây.`,
      sections: [
        { h: '1. Chấp nhận điều khoản', p: [`Khi đăng ký, đăng nhập (kể cả bằng Facebook hoặc Google) hoặc sử dụng ${APP_NAME}, bạn xác nhận đã đọc, hiểu và đồng ý với Điều khoản này và Chính sách quyền riêng tư. Nếu bạn dùng thay mặt một doanh nghiệp, bạn xác nhận có thẩm quyền ràng buộc doanh nghiệp đó. ${who('vi')}.`] },
        { h: '2. Dịch vụ', p: [`${APP_NAME} cung cấp công cụ quản lý Fanpage Facebook: hộp thư gộp, trả lời khách (có thể dùng AI), lên lịch đăng bài, theo dõi đối thủ, chăm sóc khách cũ và dựng website bán hàng. Tính năng có thể thay đổi, bổ sung hoặc ngừng theo thời gian. Chúng tôi sẽ báo trước nếu bắt đầu thu phí.`] },
        {
          h: '3. Tài khoản',
          ul: [
            'Bạn cung cấp thông tin chính xác và giữ bí mật thông tin đăng nhập; bạn chịu trách nhiệm cho hoạt động dưới tài khoản của mình.',
            'Chủ tài khoản có thể tạo tài khoản nhân viên và cấp quyền; chủ tài khoản chịu trách nhiệm về hành vi của nhân viên.',
            'Chúng tôi có thể giới hạn số Fanpage, website hoặc nhân viên cho mỗi tài khoản.'
          ]
        },
        {
          h: '4. Sử dụng được chấp nhận',
          p: ['Bạn cam kết:'],
          ul: [
            'Tuân thủ pháp luật Việt Nam, Điều khoản nền tảng của Meta (Facebook Platform Terms), Chính sách Messenger (bao gồm khung thời gian nhắn tin và quy định về gửi tin chủ động) và điều khoản của các dịch vụ bên thứ ba bạn kết nối.',
            'Chỉ kết nối Fanpage mà bạn có quyền quản lý, và có cơ sở hợp pháp (kể cả sự đồng ý khi cần) để xử lý dữ liệu khách hàng của mình.',
            'Không gửi tin rác, tin lừa đảo, nội dung vi phạm pháp luật hoặc xâm phạm quyền của người khác; không quấy rối khách hàng.',
            'Không can thiệp, dò quét, vượt qua giới hạn hoặc làm quá tải hệ thống; không dùng ứng dụng để thu thập dữ liệu trái phép.',
            'Không bán hàng giả, hàng cấm hoặc hàng hoá dịch vụ bị pháp luật hạn chế trên website bạn tạo.'
          ]
        },
        {
          h: '5. Dữ liệu và nội dung của bạn',
          p: ['Bạn giữ quyền sở hữu nội dung và dữ liệu của mình. Bạn cho chúng tôi quyền xử lý chúng ở mức cần thiết để cung cấp dịch vụ theo Chính sách quyền riêng tư. Đối với dữ liệu của khách hàng của bạn (người nhắn tin cho Fanpage, người mua hàng trên website), bạn là bên quyết định mục đích và cách xử lý, còn chúng tôi xử lý thay bạn; bạn chịu trách nhiệm thông báo và xin sự đồng ý cần thiết từ họ.']
        },
        { h: '6. Tính năng AI', p: ['Nội dung do AI tạo ra có thể sai hoặc không phù hợp. Bạn chịu trách nhiệm xem xét trước khi gửi hoặc xuất bản, đặc biệt với thông tin về giá, chính sách, sức khoẻ hoặc pháp lý. Chúng tôi không bảo đảm tính chính xác của kết quả AI.'] },
        { h: '7. Dịch vụ của bên thứ ba', p: ['Ứng dụng phụ thuộc vào Meta, Google, cổng thanh toán và các nhà cung cấp khác. Chúng tôi không kiểm soát và không chịu trách nhiệm về việc họ thay đổi, giới hạn hoặc ngừng dịch vụ (ví dụ thay đổi quyền API, thu hồi mã truy cập).'] },
        { h: '8. Sở hữu trí tuệ', p: [`${APP_NAME}, giao diện, mã nguồn và nhãn hiệu thuộc về đơn vị vận hành hoặc bên cấp phép. Bạn được cấp quyền sử dụng không độc quyền, không chuyển nhượng để dùng dịch vụ theo Điều khoản này.`] },
        { h: '9. Tạm ngưng và chấm dứt', p: ['Bạn có thể ngừng dùng dịch vụ và yêu cầu xoá dữ liệu bất cứ lúc nào (xem trang Hướng dẫn xoá dữ liệu). Chúng tôi có thể tạm ngưng hoặc chấm dứt tài khoản nếu bạn vi phạm điều khoản, gây rủi ro cho hệ thống hoặc người khác, hoặc theo yêu cầu của pháp luật hoặc Meta.'] },
        { h: '10. Miễn trừ bảo đảm', p: ['Dịch vụ được cung cấp “nguyên trạng” và “như hiện có”. Chúng tôi không bảo đảm dịch vụ không gián đoạn, không lỗi, hoặc luôn phù hợp với mục đích cụ thể của bạn.'] },
        { h: '11. Giới hạn trách nhiệm', p: ['Trong phạm vi pháp luật cho phép, chúng tôi không chịu trách nhiệm về thiệt hại gián tiếp, mất lợi nhuận, mất dữ liệu hoặc mất cơ hội kinh doanh phát sinh từ việc sử dụng dịch vụ. Tổng trách nhiệm của chúng tôi, nếu có, không vượt quá số tiền bạn đã trả cho dịch vụ trong 12 tháng trước sự kiện gây thiệt hại.'] },
        { h: '12. Luật áp dụng', p: ['Điều khoản này chịu sự điều chỉnh của pháp luật Việt Nam. Tranh chấp được giải quyết thông qua thương lượng; nếu không thành, thuộc thẩm quyền toà án có thẩm quyền tại Việt Nam.'] },
        { h: '13. Thay đổi điều khoản', p: ['Chúng tôi có thể cập nhật điều khoản này; phiên bản hiện hành là phiên bản đang hiển thị ở đây cùng ngày “Cập nhật lần cuối”. Tiếp tục sử dụng sau khi thay đổi có hiệu lực nghĩa là bạn chấp nhận.'] },
        contactSection('vi')
      ]
    }
  return {
    title: 'Terms of Service',
    summary: `By creating an account or using ${APP_NAME}, you agree to the terms below.`,
    sections: [
      { h: '1. Acceptance', p: [`By signing up, signing in (including with Facebook or Google) or using ${APP_NAME}, you confirm that you have read, understood and agree to these Terms and the Privacy Policy. If you use the service on behalf of a business, you confirm you have authority to bind it. ${who('en')}.`] },
      { h: '2. The service', p: [`${APP_NAME} provides tools for managing Facebook Pages: a unified inbox, customer replies (optionally with AI), post scheduling, competitor tracking, customer follow-up and an online store builder. Features may change, be added or be discontinued over time. We will give notice before we start charging.`] },
      {
        h: '3. Accounts',
        ul: [
          'You provide accurate information and keep your sign-in credentials confidential; you are responsible for activity under your account.',
          'An account owner may create staff accounts and grant permissions; the owner is responsible for the staff’s actions.',
          'We may limit the number of Pages, websites or staff per account.'
        ]
      },
      {
        h: '4. Acceptable use',
        p: ['You agree to:'],
        ul: [
          'Comply with applicable law (including Vietnamese law), the Meta Platform Terms, Messenger policies (including the messaging window and rules for proactive messages) and the terms of any third-party service you connect.',
          'Connect only Pages you are authorised to manage, and have a lawful basis (including consent where required) to process your customers’ data.',
          'Not send spam, scams, unlawful content or content that infringes others’ rights; not harass customers.',
          'Not interfere with, probe, bypass limits of or overload the system; not use the app to scrape data without authorisation.',
          'Not sell counterfeit, prohibited or legally restricted goods or services on websites you build.'
        ]
      },
      {
        h: '5. Your data and content',
        p: ['You keep ownership of your content and data. You grant us the right to process them as needed to provide the service, as described in the Privacy Policy. For your customers’ data (people who message your Page, buyers on your website) you decide the purposes and means of processing and we process it on your behalf; you are responsible for giving them the notices and obtaining the consents that are required.']
      },
      { h: '6. AI features', p: ['AI-generated content may be inaccurate or inappropriate. You are responsible for reviewing it before sending or publishing, especially anything about prices, policies, health or legal matters. We do not guarantee the accuracy of AI output.'] },
      { h: '7. Third-party services', p: ['The application depends on Meta, Google, payment gateways and other providers. We do not control, and are not responsible for, their changing, limiting or discontinuing their services (for example API permission changes or revoked access tokens).'] },
      { h: '8. Intellectual property', p: [`${APP_NAME}, its interface, source code and trademarks belong to the operator or its licensors. You receive a non-exclusive, non-transferable right to use the service under these Terms.`] },
      { h: '9. Suspension and termination', p: ['You may stop using the service and ask for your data to be deleted at any time (see the Data Deletion Instructions page). We may suspend or terminate an account if you breach these Terms, put the system or others at risk, or where required by law or Meta.'] },
      { h: '10. Disclaimer', p: ['The service is provided “as is” and “as available”. We do not warrant that it will be uninterrupted, error-free or fit for your particular purpose.'] },
      { h: '11. Limitation of liability', p: ['To the extent permitted by law, we are not liable for indirect damages, lost profits, lost data or lost business opportunities arising from use of the service. Our total liability, if any, will not exceed the amount you paid for the service in the 12 months before the event giving rise to the claim.'] },
      { h: '12. Governing law', p: ['These Terms are governed by the laws of Vietnam. Disputes will be resolved through negotiation and, failing that, by the competent courts in Vietnam.'] },
      { h: '13. Changes', p: ['We may update these Terms; the current version is the one shown here with its “Last updated” date. Continuing to use the service after a change takes effect means you accept it.'] },
      contactSection('en')
    ]
  }
}

// ----------------------------------------------------------------------------------------------- Xoá dữ liệu
function deletion(lang: LegalLang): LegalDoc {
  if (lang === 'vi')
    return {
      title: 'Hướng dẫn xoá dữ liệu',
      summary: `Bạn có toàn quyền yêu cầu xoá dữ liệu mà ${APP_NAME} đã lưu về bạn. Làm theo một trong các cách dưới đây.`,
      sections: [
        {
          h: 'Cách 1 — Thu hồi quyền của ứng dụng trên Facebook',
          p: [`Việc này chặn ${APP_NAME} đọc dữ liệu Facebook của bạn ngay lập tức:`],
          ul: [
            'Mở Facebook → Cài đặt & quyền riêng tư → Cài đặt.',
            'Chọn "Ứng dụng và trang web" (hoặc "Tích hợp doanh nghiệp").',
            `Tìm "${APP_NAME}" → bấm "Xoá" (Remove) và xác nhận.`
          ],
          after: ['Sau bước này ứng dụng không còn truy cập được Fanpage và tin nhắn của bạn từ Facebook. Dữ liệu đã đồng bộ về PagePilot vẫn còn cho tới khi bạn làm Cách 2 hoặc Cách 3.']
        },
        {
          h: 'Cách 2 — Gỡ Fanpage khỏi ứng dụng',
          ul: [`Đăng nhập ${APP_NAME} → mục Fanpage → chọn Fanpage cần gỡ → bấm "Gỡ Page".`],
          after: ['Mã truy cập Fanpage đã lưu bị xoá ngay và ứng dụng ngừng nhận tin nhắn của Fanpage đó. Các cuộc trò chuyện đã đồng bộ trước đó được xoá khi bạn yêu cầu theo Cách 3.']
        },
        {
          h: 'Cách 3 — Yêu cầu xoá tài khoản và toàn bộ dữ liệu',
          p: contact.email
            ? [`Gửi email tới ${contact.email} với tiêu đề "Yêu cầu xoá dữ liệu / Data deletion request", từ địa chỉ email đã đăng ký tài khoản (hoặc ghi rõ email đăng ký và tên Facebook của bạn để chúng tôi xác minh).`]
            : ['Liên hệ đơn vị vận hành ứng dụng qua kênh hỗ trợ đã cung cấp cho bạn khi đăng ký, với tiêu đề "Yêu cầu xoá dữ liệu / Data deletion request", kèm email đăng ký và tên Facebook của bạn để chúng tôi xác minh.'],
          ul: [
            'Chúng tôi xác nhận yêu cầu và xoá dữ liệu trong vòng 30 ngày, sau đó gửi thông báo hoàn tất.',
            'Dữ liệu bị xoá gồm: tài khoản và nhân viên, Fanpage đã kết nối cùng mã truy cập, hội thoại và tin nhắn đã đồng bộ, ghi chú CRM, bài đăng đã lên lịch, cài đặt AI, chiến dịch chăm sóc, danh sách đối thủ, website, sản phẩm, đơn hàng và thống kê truy cập của bạn.',
            'Dữ liệu pháp luật bắt buộc phải giữ (ví dụ chứng từ giao dịch) có thể được giữ lại trong thời hạn luật định rồi xoá.'
          ]
        },
        {
          h: 'Nếu bạn từng nhắn tin cho một Fanpage dùng PagePilot',
          p: [contact.email ? `Hãy gửi email tới ${contact.email} kèm tên Facebook của bạn và tên Fanpage đã nhắn tin (hoặc liên hệ chủ Fanpage). Chúng tôi sẽ xoá các cuộc trò chuyện gắn với bạn trong vòng 30 ngày.` : 'Hãy liên hệ chủ Fanpage hoặc đơn vị vận hành ứng dụng, kèm tên Facebook của bạn và tên Fanpage đã nhắn tin. Chúng tôi sẽ xoá các cuộc trò chuyện gắn với bạn trong vòng 30 ngày.']
        },
        { h: 'Dữ liệu tự động bị xoá', p: ['Sự kiện thống kê truy cập website tự xoá sau 60 ngày và phiên truy cập sau 180 ngày; các dữ liệu này không chứa tên, số điện thoại, email hay địa chỉ IP.'] },
        contactSection('vi')
      ]
    }
  return {
    title: 'Data Deletion Instructions',
    summary: `You have the right to ask for the data ${APP_NAME} has stored about you to be deleted. Use any of the options below.`,
    sections: [
      {
        h: 'Option 1 — Remove the app from your Facebook account',
        p: [`This immediately stops ${APP_NAME} from reading your Facebook data:`],
        ul: [
          'Open Facebook → Settings & privacy → Settings.',
          'Choose “Apps and Websites” (or “Business Integrations”).',
          `Find “${APP_NAME}” → click “Remove” and confirm.`
        ],
        after: ['After this the app can no longer access your Pages and messages from Facebook. Data already synced into PagePilot remains until you complete Option 2 or Option 3.']
      },
      {
        h: 'Option 2 — Remove a Page from the app',
        ul: [`Sign in to ${APP_NAME} → Pages → select the Page → click “Remove Page”.`],
        after: ['The stored Page access token is deleted immediately and the app stops receiving that Page’s messages. Conversations synced earlier are deleted when you request it under Option 3.']
      },
      {
        h: 'Option 3 — Ask us to delete your account and all data',
        p: contact.email
          ? [`Email ${contact.email} with the subject “Data deletion request”, from the email address registered to your account (or state your registered email and your Facebook name so we can verify you).`]
          : ['Contact the operator of the application through the support channel provided to you when you signed up, with the subject “Data deletion request”, and include your registered email and Facebook name so we can verify you.'],
        ul: [
          'We confirm the request and delete the data within 30 days, then notify you when it is done.',
          'Deleted data includes: your account and staff, connected Pages and their access tokens, synced conversations and messages, CRM notes, scheduled posts, AI settings, care campaigns, tracked competitors, websites, products, orders and traffic statistics.',
          'Data we are legally required to keep (for example transaction records) may be retained for the statutory period and then deleted.'
        ]
      },
      {
        h: 'If you messaged a Page that uses PagePilot',
        p: [contact.email ? `Email ${contact.email} with your Facebook name and the name of the Page you messaged (or contact the Page owner). We will delete the conversations linked to you within 30 days.` : 'Contact the Page owner or the operator of the application with your Facebook name and the name of the Page you messaged. We will delete the conversations linked to you within 30 days.']
      },
      { h: 'Data that is deleted automatically', p: ['Website traffic events are deleted automatically after 60 days and visit sessions after 180 days; this data contains no names, phone numbers, emails or IP addresses.'] },
      contactSection('en')
    ]
  }
}

export function legalDoc(key: LegalKey, lang: LegalLang): LegalDoc {
  return key === 'privacy' ? privacy(lang) : key === 'terms' ? terms(lang) : deletion(lang)
}
