/** A business, as the API returns it. */
export interface Business {
  id: number;
  name: string;
  created_at: string;
}

export interface BusinessListResponse {
  items: Business[];
}
