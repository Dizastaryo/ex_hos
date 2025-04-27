import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'product_detail_screen.dart';
import '../services/product_service.dart';
import '../services/category_service.dart';
import '../models/product.dart';
import '../models/category.dart';

class ProductsScreen extends StatefulWidget {
  const ProductsScreen({Key? key}) : super(key: key);

  @override
  State<ProductsScreen> createState() => _ProductsScreenState();
}

class _ProductsScreenState extends State<ProductsScreen> {
  late Future<List<Product>> _futureProducts;
  List<Category> _categories = [];
  Category? _selectedCategory;
  String? _searchText;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadCategories();
      _loadProducts();
    });
  }

  void _loadProducts() {
    final productService = Provider.of<ProductService>(context, listen: false);
    setState(() {
      _futureProducts = productService.getProducts(
        categoryId: _selectedCategory?.id,
        search: _searchText,
      );
    });
  }

  Future<void> _loadCategories() async {
    try {
      final service = Provider.of<CategoryService>(context, listen: false);
      final cats = await service.getCategories();
      setState(() => _categories = cats);
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Ошибка загрузки категорий: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Продукты'),
        backgroundColor: const Color(0xFF6A0DAD),
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(8.0),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    decoration: InputDecoration(
                      hintText: 'Поиск товара...',
                      prefixIcon: const Icon(Icons.search),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      filled: true,
                    ),
                    textInputAction: TextInputAction.search,
                    onSubmitted: (text) {
                      _searchText = text.trim();
                      _loadProducts();
                    },
                  ),
                ),
                const SizedBox(width: 8),
                DropdownButton<Category>(
                  value: _selectedCategory,
                  hint: const Text('Категория'),
                  items: [
                    const DropdownMenuItem<Category>(
                      value: null,
                      child: Text('Все'),
                    ),
                    ..._categories.map((c) => DropdownMenuItem<Category>(
                          value: c,
                          child: Text(c.name),
                        )),
                  ],
                  onChanged: (cat) {
                    setState(() => _selectedCategory = cat);
                    _loadProducts();
                  },
                ),
              ],
            ),
          ),
          Expanded(
            child: FutureBuilder<List<Product>>(
              future: _futureProducts,
              builder: (context, snapshot) {
                if (snapshot.connectionState == ConnectionState.waiting) {
                  return const Center(child: CircularProgressIndicator());
                } else if (snapshot.hasError) {
                  return Center(child: Text('Ошибка: ${snapshot.error}'));
                } else if (!snapshot.hasData || snapshot.data!.isEmpty) {
                  return const Center(child: Text('Нет продуктов.'));
                }

                final products = snapshot.data!;
                return Padding(
                  padding: const EdgeInsets.all(8.0),
                  child: GridView.builder(
                    gridDelegate:
                        const SliverGridDelegateWithFixedCrossAxisCount(
                      crossAxisCount: 2,
                      crossAxisSpacing: 8,
                      mainAxisSpacing: 8,
                      childAspectRatio: 0.75,
                    ),
                    itemCount: products.length,
                    itemBuilder: (context, index) {
                      final product = products[index];
                      final imageUrl = product.imageUrls.isNotEmpty
                          ? 'http://172.20.10.2:8000${product.imageUrls.first}'
                          : 'https://via.placeholder.com/150';
                      return GestureDetector(
                        onTap: () {
                          Navigator.push(
                            context,
                            MaterialPageRoute(
                              builder: (_) =>
                                  ProductDetailScreen(productId: product.id),
                            ),
                          );
                        },
                        child: Card(
                          elevation: 4,
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Expanded(
                                child: ClipRRect(
                                  borderRadius: const BorderRadius.only(
                                    topLeft: Radius.circular(12),
                                    topRight: Radius.circular(12),
                                  ),
                                  child: Image.network(
                                    imageUrl,
                                    width: double.infinity,
                                    fit: BoxFit.cover,
                                  ),
                                ),
                              ),
                              Padding(
                                padding: const EdgeInsets.all(8.0),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      product.name,
                                      style: const TextStyle(
                                        fontSize: 16,
                                        fontWeight: FontWeight.bold,
                                      ),
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      '${product.price.toStringAsFixed(2)} ₸',
                                      style: const TextStyle(fontSize: 14),
                                    ),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
