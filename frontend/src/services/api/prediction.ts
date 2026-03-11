import apiClient from './client';
import { ListPredictionResponse, GetPredictionDetailResponse } from '../../types/prediction';

export const predictionService = {
  getPredictionList: async (page: number = 1, size: number = 10) => {
    return apiClient.get<any, ListPredictionResponse>('/prediction/list', { 
      params: { page, size } 
    });
  },

  getPredictionDetail: async (id: number) => {
    // The user gave `http://localhost:8888/api/prediction/2` which implies GET /prediction/:id
    return apiClient.get<any, GetPredictionDetailResponse>(`/prediction/${id}`);
  }
};
