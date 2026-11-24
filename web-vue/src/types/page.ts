export interface FacebookPageItem {
  id: number
  workspace_id: number
  page_id: string
  name: string
  category: string
  avatar_url: string
  is_connected: boolean
  webhook_subscribed: boolean
  unread_count: number
  created_at?: string
}