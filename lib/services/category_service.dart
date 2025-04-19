import 'package:dio/dio.dart';
import '../models/category.dart';

class CategoryService {
  final Dio _dio = Dio(BaseOptions(baseUrl: 'http://10.0.2.2:8000/'));

  Future<List<Category>> getCategories() async {
    final response = await _dio.get('/categories/');
    return (response.data as List)
        .map((json) => Category.fromJson(json))
        .toList();
  }
}
