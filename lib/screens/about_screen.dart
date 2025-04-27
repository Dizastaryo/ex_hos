import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/product_service.dart';
import '../models/product.dart';
import 'add_product_screen.dart';
import 'update_product_screen.dart';

class AboutScreen extends StatefulWidget {
  const AboutScreen({super.key});

  @override
  State<AboutScreen> createState() => _AboutScreenState();
}

class _AboutScreenState extends State<AboutScreen> {
  late Future<List<Product>> _futureProducts;

  @override
  void initState() {
    super.initState();
    _loadProducts();
  }

  void _loadProducts() {
    final productService = Provider.of<ProductService>(context, listen: false);
    setState(() {
      _futureProducts = productService.getProducts();
    });
  }

  void _showProductOptions(BuildContext context, Product product) {
    showModalBottomSheet(
      context: context,
      builder: (ctx) => Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            leading: const Icon(Icons.edit),
            title: const Text('Редактировать'),
            onTap: () {
              Navigator.pop(ctx);
              _navigateToUpdateScreen(context, product);
            },
          ),
          ListTile(
            leading: const Icon(Icons.delete),
            title: const Text('Удалить'),
            onTap: () {
              Navigator.pop(ctx);
              _confirmDeleteProduct(context, product.id);
            },
          ),
        ],
      ),
    );
  }

  void _navigateToUpdateScreen(BuildContext context, Product product) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (ctx) => UpdateProductScreen(
          productId: product.id,
          product: product,
        ),
      ),
    ).then((_) => _loadProducts());
  }

  void _confirmDeleteProduct(BuildContext context, int productId) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Подтверждение удаления'),
        content: const Text('Вы уверены, что хотите удалить этот продукт?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Отмена'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              await _deleteProduct(productId);
            },
            child: const Text('Удалить', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
  }

  Future<void> _deleteProduct(int productId) async {
    try {
      final service = Provider.of<ProductService>(context, listen: false);
      await service.deleteProduct(productId);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Продукт успешно удален')),
      );
      _loadProducts();
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка удаления: $e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Управление продуктами'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadProducts,
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (ctx) => const AddProductScreen()),
        ).then((_) => _loadProducts()),
        child: const Icon(Icons.add),
      ),
      body: FutureBuilder<List<Product>>(
        future: _futureProducts,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }

          if (snapshot.hasError) {
            return Center(child: Text('Ошибка загрузки: ${snapshot.error}'));
          }

          if (!snapshot.hasData || snapshot.data!.isEmpty) {
            return const Center(child: Text('Список продуктов пуст'));
          }

          final products = snapshot.data!;
          return _buildProductsGrid(products);
        },
      ),
    );
  }

  Widget _buildProductsGrid(List<Product> products) {
    return Padding(
      padding: const EdgeInsets.all(8.0),
      child: GridView.builder(
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 2,
          crossAxisSpacing: 8,
          mainAxisSpacing: 8,
          childAspectRatio: 0.75,
        ),
        itemCount: products.length,
        itemBuilder: (context, index) {
          final product = products[index];
          return _buildProductCard(product);
        },
      ),
    );
  }

  Widget _buildProductCard(Product product) {
    final productService = Provider.of<ProductService>(context, listen: false);
    final imgUrl = product.imageUrls.isNotEmpty
        ? productService.getImageUrl(product.imageUrls.first)
        : 'https://via.placeholder.com/150';

    return GestureDetector(
      onTap: () => _showProductOptions(context, product),
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
                borderRadius:
                    const BorderRadius.vertical(top: Radius.circular(12)),
                child: Image.network(
                  imgUrl,
                  width: double.infinity,
                  fit: BoxFit.cover,
                  errorBuilder: (ctx, error, stackTrace) =>
                      const Icon(Icons.error_outline),
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
  }
}
