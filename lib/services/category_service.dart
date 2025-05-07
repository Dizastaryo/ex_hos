import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';
import '../models/category.dart';

class CategoryService {
  // Берём базовый URL из .env
  final String _baseUrl = dotenv.env['API_BASE_URL']!;
  final Dio _dio;

  CategoryService(this._dio);

  Future<List<Category>> getCategories() async {
    try {
      final response = await _dio.get('$_baseUrl/categories/');
      return (response.data as List)
          .map((json) => Category.fromJson(json))
          .toList();
    } on DioException catch (e) {
      throw e.response != null
          ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
          : 'Сетевая ошибка: ${e.message}';
    }
  }
}
