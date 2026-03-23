import apiClient from './client';
import { ListPredictionResponse, GetPredictionDetailResponse } from '../../types/prediction';
import { PREDICTION_API } from '@/config/api';

export const predictionService = {
  getPredictionList: async (page: number = 1, size: number = 10) => {
    return apiClient.get<any, ListPredictionResponse>(PREDICTION_API.LIST, { 
      params: { page, size } 
    });
  },

  getPredictionDetail: async (id: number) => {
    // The user gave `http://localhost:8899/api/prediction/2` which implies GET /prediction/:id
    return apiClient.get<any, GetPredictionDetailResponse>(PREDICTION_API.DETAIL(id));
  },

  deleteHistory: async (ids: number[]) => {
    return apiClient.post(PREDICTION_API.DELETE, { ids });
  }
};
