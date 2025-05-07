import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';
import '../models/product.dart';
import '../models/review.dart';

class ProductService {
  final Dio _dio;
  // Берём базовый URL из .env
  final String _baseUrl = dotenv.env['API_BASE_URL']!;

  ProductService(this._dio);

  Future<Product> addProduct({
    required ProductCreate product,
    required List<File> images,
  }) async {
    try {
      final formData = await _buildFormData(product, images);
      final response = await _dio.post(
        '$_baseUrl/products/',
        data: formData,
        options: Options(contentType: 'multipart/form-data'),
      );
      return Product.fromJson(response.data);
    } on DioException catch (e) {
      throw _formatError(e);
    } catch (e) {
      throw 'Ошибка при создании продукта: $e';
    }
  }

  Future<List<Product>> getProducts({int? categoryId, String? search}) async {
    try {
      final response = await _dio.get(
        '$_baseUrl/products/',
        queryParameters: {
          if (categoryId != null) 'category_id': categoryId,
          if (search != null && search.isNotEmpty) 'search': search,
        },
      );
      return (response.data as List)
          .map((item) => Product.fromJson(item))
          .toList();
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<Product> updateProduct({
    required int id,
    required ProductCreate product,
    required List<File> images,
  }) async {
    try {
      final formData = await _buildFormData(product, images);
      final response = await _dio.put(
        '$_baseUrl/products/$id',
        data: formData,
        options: Options(contentType: 'multipart/form-data'),
      );
      return Product.fromJson(response.data);
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<void> deleteProduct(int id) async {
    try {
      await _dio.delete('$_baseUrl/products/$id');
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<FormData> _buildFormData(
      ProductCreate product, List<File> images) async {
    final imageFiles = await Future.wait(images.map(
      (file) => MultipartFile.fromFile(
        file.path,
        filename: file.path.split('/').last,
      ),
    ));
    return FormData.fromMap({
      ...product.toJson(),
      'images': imageFiles,
    });
  }

  Future<Product> getProductById(int id) async {
    try {
      final response = await _dio.get('$_baseUrl/products/$id');
      return Product.fromJson(response.data);
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  Future<Map<String, dynamic>> getCart() async {
    final response = await _dio.get('$_baseUrl/cart/');
    return response.data as Map<String, dynamic>;
  }

  Future<void> addToCart(int productId, {int quantity = 1}) async {
    await _dio.post(
      '$_baseUrl/cart/add',
      data: {'product_id': productId, 'quantity': quantity},
    );
  }

  Future<void> updateCart(int productId, int quantity) async {
    await _dio.put(
      '$_baseUrl/cart/update',
      data: {'product_id': productId, 'quantity': quantity},
    );
  }

  Future<void> removeFromCart(int productId) async {
    await _dio.delete('$_baseUrl/cart/remove/$productId');
  }

  Future<void> clearCart() async {
    await _dio.delete('$_baseUrl/cart/clear');
  }

  Future<Review> addReview({
    required int productId,
    required int rating,
    String? comment,
  }) async {
    final resp = await _dio.post(
      '$_baseUrl/reviews/',
      data: {
        'product_id': productId,
        'rating': rating,
        'comment': comment,
      },
    );
    return Review.fromJson(resp.data);
  }

  Future<List<Review>> getReviewsForProduct(int productId) async {
    final resp = await _dio.get('$_baseUrl/reviews/product/$productId');
    return (resp.data as List).map((e) => Review.fromJson(e)).toList();
  }

  String getImageUrl(String imagePath) {
    return '$_baseUrl$imagePath';
  }

  String _formatError(DioException e) {
    return e.response != null
        ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
        : 'Сетевая ошибка: ${e.message}';
  }
}
