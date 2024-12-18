import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/box_provider.dart';
import '../models/box_model.dart';
import 'package:model_viewer_plus/model_viewer_plus.dart';
import 'box_screen.dart';

class BoxSelectionScreen extends StatefulWidget {
  const BoxSelectionScreen({super.key});

  @override
  State<BoxSelectionScreen> createState() => _BoxSelectionScreenState();
}

class _BoxSelectionScreenState extends State<BoxSelectionScreen>
    with TickerProviderStateMixin {
  BoxModel? selectedBox;
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 6, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final boxes = Provider.of<BoxProvider>(context).boxes;

    return Scaffold(
      appBar: AppBar(
        title: const Text(
          'Карта склада',
          style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
        ),
        centerTitle: true,
        backgroundColor: const Color(0xFF4CAF50),
        elevation: 0,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(bottom: Radius.circular(16)),
        ),
      ),
      body: Column(
        children: [
          // Таб-секция
          Container(
            color: Colors.white,
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: TabBar(
              controller: _tabController,
              isScrollable: true,
              indicator: BoxDecoration(
                color: const Color(0xFF4CAF50),
                borderRadius: BorderRadius.circular(16),
              ),
              labelColor: Colors.white,
              unselectedLabelColor: Colors.black54,
              labelStyle:
                  const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
              tabs: const [
                Tab(text: 'XXS'),
                Tab(text: 'XS'),
                Tab(text: 'S'),
                Tab(text: 'M'),
                Tab(text: 'L'),
                Tab(text: 'XL'),
              ],
            ),
          ),
          // Список боксов
          Expanded(
            flex: 2,
            child: TabBarView(
              controller: _tabController,
              children: [
                _buildBoxGrid(context, boxes, BoxType.xxs),
                _buildBoxGrid(context, boxes, BoxType.xs),
                _buildBoxGrid(context, boxes, BoxType.s),
                _buildBoxGrid(context, boxes, BoxType.m),
                _buildBoxGrid(context, boxes, BoxType.l),
                _buildBoxGrid(context, boxes, BoxType.xl),
              ],
            ),
          ),
          // Модель бокса
          if (selectedBox != null)
            GestureDetector(
              onTap: () => _show3DModelDialog(context, selectedBox!),
              child: Container(
                margin: const EdgeInsets.symmetric(horizontal: 16),
                height: 300,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(16),
                  gradient: const LinearGradient(
                    colors: [Color(0xFFE8F5E9), Color(0xFFDCEDC8)],
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                  ),
                  boxShadow: const [
                    BoxShadow(
                      color: Colors.black26,
                      blurRadius: 12,
                      offset: Offset(0, 6),
                    ),
                  ],
                ),
                child: ModelViewer(
                  key: ValueKey(selectedBox!.id),
                  src:
                      "assets/3d_models/${selectedBox!.type.name.toUpperCase()}.glb",
                  autoRotate: true,
                  cameraControls: true, // Enable manual rotation
                  backgroundColor: const Color(0xFFE8F5E9),
                  alt: "3D модель ${selectedBox!.type.name.toUpperCase()}",
                ),
              ),
            ),
          // Кнопка "Арендовать"
          Padding(
            padding:
                const EdgeInsets.symmetric(horizontal: 16.0, vertical: 8.0),
            child: ElevatedButton(
              onPressed: selectedBox != null
                  ? () {
                      Navigator.push(
                        context,
                        MaterialPageRoute(
                          builder: (context) => BoxScreen(box: selectedBox!),
                        ),
                      );
                    }
                  : null,
              style: ElevatedButton.styleFrom(
                backgroundColor:
                    selectedBox != null ? const Color(0xFF4CAF50) : Colors.grey,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(16),
                ),
                elevation: 4,
              ),
              child: const Text(
                'Арендовать',
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                  color: Colors.white,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBoxGrid(
      BuildContext context, List<BoxModel> boxes, BoxType type) {
    final filteredBoxes = boxes.where((box) => box.type == type).toList();

    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: GridView.builder(
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 4,
          crossAxisSpacing: 12,
          mainAxisSpacing: 12,
        ),
        itemCount: filteredBoxes.length,
        itemBuilder: (context, index) {
          final box = filteredBoxes[index];
          return GestureDetector(
            onTap: box.isAvailable
                ? () {
                    setState(() {
                      selectedBox = box;
                    });
                  }
                : null,
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 300),
              decoration: BoxDecoration(
                color: box.isAvailable ? const Color(0xFF81C784) : Colors.red,
                borderRadius: BorderRadius.circular(12),
                border: selectedBox == box
                    ? Border.all(color: Colors.black, width: 2)
                    : null,
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withOpacity(0.1),
                    blurRadius: 6,
                    offset: const Offset(0, 4),
                  ),
                ],
              ),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(
                    box.id,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  if (box.isAvailable)
                    const Icon(Icons.check_circle, color: Colors.white)
                  else
                    const Icon(Icons.block, color: Colors.white),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  void _show3DModelDialog(BuildContext context, BoxModel box) {
    showDialog(
      context: context,
      builder: (context) {
        return Dialog(
          child: Container(
            height: 400,
            child: ModelViewer(
              src: "assets/3d_models/${box.type.name.toUpperCase()}.glb",
              autoRotate: true,
              cameraControls: true, // Enable manual rotation
            ),
          ),
        );
      },
    );
  }
}
