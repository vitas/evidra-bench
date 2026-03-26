export interface EnabledModel {
  id: string;
  display_name: string;
  provider: string;
  input_cost_per_mtok: number;
  output_cost_per_mtok: number;
}

export interface ModelsResponse {
  models: EnabledModel[];
}
