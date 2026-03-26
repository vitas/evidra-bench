export interface EnabledModel {
  id: string;
  display_name: string;
  provider: string;
  api_base_url?: string;
  available: boolean;
  input_cost_per_mtok: number;
  output_cost_per_mtok: number;
}

export interface ModelsResponse {
  models: EnabledModel[];
}
