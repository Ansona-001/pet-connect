import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_exception.dart';
import '../../../core/realtime/realtime_client.dart';
import '../../auth/application/auth_controller.dart';
import '../data/mock_social_data.dart';
import '../data/social_repository.dart';
import '../domain/social_models.dart';

class SocialState {
  const SocialState({
    required this.posts,
    required this.stories,
    required this.candidates,
    required this.pets,
    required this.chats,
    required this.candidateIndex,
    required this.messagesByChat,
    required this.userProfile,
    required this.activePetIndex,
    required this.readChatIds,
    required this.superLikedPetIds,
    required this.profileStats,
    this.matchRadiusKm = 10,
    this.lastMatch,
    this.lastMatchChatId,
    this.isLoading = false,
    this.initialized = false,
    this.error,
  });

  factory SocialState.initial({bool useMocks = true}) => SocialState(
    posts: useMocks ? MockSocialData.posts : const [],
    stories: useMocks ? MockSocialData.stories : const [],
    candidates: useMocks ? MockSocialData.matchCandidates : const [],
    pets: const [],
    chats: useMocks ? MockSocialData.chats : const [],
    candidateIndex: 0,
    messagesByChat: useMocks
        ? const {
            'chat-coco': MockSocialData.messages,
            'chat-milo': [
              ChatMessage(
                id: 'milo-1',
                text: 'That trail looks perfect.',
                time: '2:10 PM',
                isMine: false,
              ),
            ],
            'chat-simba': [
              ChatMessage(
                id: 'simba-1',
                text: 'Simba finally approved the new cat tree 😸',
                time: 'Yesterday',
                isMine: false,
              ),
            ],
          }
        : const {},
    userProfile: const UserSetupProfile.demo(),
    activePetIndex: 0,
    readChatIds: const <String>{},
    superLikedPetIds: const <String>{},
    profileStats: useMocks
        ? const ProfileStats(
            postCount: 128,
            followerCount: 12800,
            followingCount: 486,
            petCount: 2,
          )
        : const ProfileStats.zero(),
    initialized: useMocks,
  );

  final List<FeedPost> posts;
  final List<PetStory> stories;
  final List<PetProfile> candidates;
  final List<PetProfile> pets;
  final List<ChatPreview> chats;
  final int candidateIndex;
  final Map<String, List<ChatMessage>> messagesByChat;
  final UserSetupProfile userProfile;
  final int activePetIndex;
  final Set<String> readChatIds;
  final Set<String> superLikedPetIds;
  final ProfileStats profileStats;
  final double matchRadiusKm;
  final PetProfile? lastMatch;
  final String? lastMatchChatId;
  final bool isLoading;
  final bool initialized;
  final String? error;

  List<ChatMessage> messagesFor(String chatId) {
    return messagesByChat[chatId] ?? const <ChatMessage>[];
  }

  SocialState copyWith({
    List<FeedPost>? posts,
    List<PetStory>? stories,
    List<PetProfile>? candidates,
    List<PetProfile>? pets,
    List<ChatPreview>? chats,
    int? candidateIndex,
    Map<String, List<ChatMessage>>? messagesByChat,
    UserSetupProfile? userProfile,
    int? activePetIndex,
    Set<String>? readChatIds,
    Set<String>? superLikedPetIds,
    ProfileStats? profileStats,
    double? matchRadiusKm,
    PetProfile? lastMatch,
    String? lastMatchChatId,
    bool clearLastMatch = false,
    bool? isLoading,
    bool? initialized,
    String? error,
    bool clearError = false,
  }) {
    final nextMessages = messagesByChat ?? this.messagesByChat;
    return SocialState(
      posts: List<FeedPost>.unmodifiable(posts ?? this.posts),
      stories: List<PetStory>.unmodifiable(stories ?? this.stories),
      candidates: List<PetProfile>.unmodifiable(candidates ?? this.candidates),
      pets: List<PetProfile>.unmodifiable(pets ?? this.pets),
      chats: List<ChatPreview>.unmodifiable(chats ?? this.chats),
      candidateIndex: candidateIndex ?? this.candidateIndex,
      messagesByChat: Map<String, List<ChatMessage>>.unmodifiable(
        nextMessages.map(
          (chatId, messages) =>
              MapEntry(chatId, List<ChatMessage>.unmodifiable(messages)),
        ),
      ),
      userProfile: userProfile ?? this.userProfile,
      activePetIndex: activePetIndex ?? this.activePetIndex,
      readChatIds: Set<String>.unmodifiable(readChatIds ?? this.readChatIds),
      superLikedPetIds: Set<String>.unmodifiable(
        superLikedPetIds ?? this.superLikedPetIds,
      ),
      profileStats: profileStats ?? this.profileStats,
      matchRadiusKm: matchRadiusKm ?? this.matchRadiusKm,
      lastMatch: clearLastMatch ? null : lastMatch ?? this.lastMatch,
      lastMatchChatId: clearLastMatch
          ? null
          : lastMatchChatId ?? this.lastMatchChatId,
      isLoading: isLoading ?? this.isLoading,
      initialized: initialized ?? this.initialized,
      error: clearError ? null : error ?? this.error,
    );
  }
}

class SocialController extends StateNotifier<SocialState> {
  SocialController({
    SocialRepository? repository,
    this.realtime,
    this.currentUserId,
    UserSetupProfile? initialProfile,
  }) : _repository = repository,
       super(SocialState.initial(useMocks: repository == null)) {
    if (initialProfile != null) {
      state = state.copyWith(userProfile: initialProfile);
    }
  }

  final SocialRepository? _repository;
  final RealtimeClient? realtime;
  final String? currentUserId;
  StreamSubscription<RealtimeEvent>? _realtimeSubscription;
  int _messageSequence = 0;

  /// Fetches the real Posts/Followers/Following/Pets counts for
  /// [currentUserId] — see server/internal/modules/profile/profile.go's
  /// `getPublicProfile`. Failures here fall back to the previous counts
  /// rather than raising, so a stats hiccup never blocks feed/pets/chat
  /// loading (which is what [initialize] is really for).
  Future<ProfileStats> _fetchProfileStats(SocialRepository repository) async {
    final userId = currentUserId;
    if (userId == null || userId.isEmpty) return state.profileStats;
    try {
      return await repository.fetchProfileStats(userId);
    } catch (_) {
      return state.profileStats;
    }
  }

  Future<void> initialize({bool force = false}) async {
    final repository = _repository;
    if (repository == null ||
        state.isLoading ||
        (state.initialized && !force)) {
      return;
    }
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final snapshot = await repository.load(radiusKm: state.matchRadiusKm);
      final primaryPet = snapshot.pets.isEmpty ? null : snapshot.pets.first;
      final profile = primaryPet == null
          ? state.userProfile
          : UserSetupProfile(
              ownerName: state.userProfile.ownerName,
              city: state.userProfile.city,
              petName: primaryPet.name,
              petType: primaryPet.petType,
              interests: primaryPet.interests,
            );
      final profileStats = await _fetchProfileStats(repository);
      state = state.copyWith(
        posts: snapshot.posts,
        stories: snapshot.stories,
        pets: snapshot.pets,
        candidates: snapshot.candidates,
        chats: snapshot.chats,
        candidateIndex: 0,
        userProfile: profile,
        profileStats: profileStats,
        isLoading: false,
        initialized: true,
        clearError: true,
      );
      _realtimeSubscription ??= realtime?.events.listen(_handleRealtimeEvent);
      unawaited(realtime?.start());
    } catch (error) {
      state = state.copyWith(
        isLoading: false,
        initialized: true,
        error: ApiException.from(error).message,
      );
    }
  }

  Future<void> toggleLike(String postId) async {
    final original = state.posts.firstWhere((post) => post.id == postId);
    final nextValue = !original.isLiked;
    _replacePost(
      original.copyWith(
        isLiked: nextValue,
        likes: original.likes + (nextValue ? 1 : -1),
      ),
    );
    try {
      await _repository?.setPostLiked(postId, nextValue);
    } catch (error) {
      _replacePost(original);
      state = state.copyWith(error: ApiException.from(error).message);
    }
  }

  Future<void> toggleSave(String postId) async {
    final original = state.posts.firstWhere((post) => post.id == postId);
    final nextValue = !original.isSaved;
    _replacePost(original.copyWith(isSaved: nextValue));
    try {
      await _repository?.setPostSaved(postId, nextValue);
    } catch (error) {
      _replacePost(original);
      state = state.copyWith(error: ApiException.from(error).message);
    }
  }

  /// Follows/unfollows the pet behind [post] — see
  /// server/internal/modules/social/handlers.go's `setFollow`. Following
  /// is pet-scoped, not post-scoped, so every post from that same pet
  /// currently in [SocialState.posts] flips together, not just the one
  /// the user tapped from.
  Future<void> toggleFollow(FeedPost post) async {
    if (post.petId.isEmpty) return;
    final nextValue = !post.isFollowedByMe;
    final originals = state.posts.where((item) => item.petId == post.petId);
    _setFollowedForPet(post.petId, nextValue);
    try {
      await _repository?.setPetFollowed(post.petId, nextValue);
    } catch (error) {
      for (final original in originals) {
        _replacePost(original);
      }
      state = state.copyWith(error: ApiException.from(error).message);
    }
  }

  void _setFollowedForPet(String petId, bool isFollowedByMe) {
    state = state.copyWith(
      posts: state.posts
          .map(
            (post) => post.petId == petId
                ? post.copyWith(isFollowedByMe: isFollowedByMe)
                : post,
          )
          .toList(growable: false),
    );
  }

  /// Creates a post from media ids the composer has already uploaded
  /// (one or many, in the given order) — see
  /// SocialRepository.createCarouselPost. The composer owns uploading
  /// each item itself (so it can show per-item progress via
  /// SocialRepository.uploadMedia directly); this only creates the post
  /// once every item has a media id.
  Future<bool> createCarouselPost({
    required String caption,
    required List<String> mediaIds,
  }) async {
    final repository = _repository;
    if (repository == null || state.pets.isEmpty || mediaIds.isEmpty) {
      return false;
    }
    final activeIndex = state.activePetIndex < state.pets.length
        ? state.activePetIndex
        : 0;
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final post = await repository.createCarouselPost(
        petId: state.pets[activeIndex].id,
        caption: caption,
        mediaIds: mediaIds,
        locationName: state.userProfile.city,
      );
      state = state.copyWith(posts: [post, ...state.posts], isLoading: false);
      return true;
    } catch (error) {
      state = state.copyWith(
        isLoading: false,
        error: ApiException.from(error).message,
      );
      return false;
    }
  }

  Future<List<String>> loadComments(String postId) async {
    final repository = _repository;
    if (repository == null) return const [];
    try {
      return await repository.comments(postId);
    } catch (error) {
      state = state.copyWith(error: ApiException.from(error).message);
      return const [];
    }
  }

  Future<String?> addComment(String postId, String body) async {
    final repository = _repository;
    if (repository == null) return body.trim();
    try {
      final comment = await repository.addComment(postId, body);
      final post = state.posts.firstWhere((item) => item.id == postId);
      _replacePost(post.copyWith(comments: post.comments + 1));
      return comment;
    } catch (error) {
      state = state.copyWith(error: ApiException.from(error).message);
      return null;
    }
  }

  void _replacePost(FeedPost replacement) {
    state = state.copyWith(
      posts: state.posts
          .map((post) => post.id == replacement.id ? replacement : post)
          .toList(growable: false),
    );
  }

  void completeSetup({
    required String ownerName,
    required String city,
    required String petName,
    required String petType,
    required List<String> interests,
  }) {
    state = state.copyWith(
      userProfile: UserSetupProfile(
        ownerName: ownerName.trim().isEmpty ? 'Pet parent' : ownerName.trim(),
        city: city.trim().isEmpty ? 'Nearby' : city.trim(),
        petName: petName.trim().isEmpty ? 'My pet' : petName.trim(),
        petType: petType,
        interests: List<String>.unmodifiable(interests),
      ),
      activePetIndex: 0,
    );
  }

  /// Re-syncs [SocialState.pets] after the "My Pets" management screens
  /// add/edit/delete a pet — see [SocialRepository.fetchMyPets]. Clamps
  /// [SocialState.activePetIndex] if the previously active pet no longer
  /// exists at that position (e.g. it was just deleted).
  Future<void> refreshPets() async {
    final repository = _repository;
    if (repository == null) return;
    try {
      final pets = await repository.fetchMyPets();
      final clampedIndex = pets.isEmpty
          ? 0
          : state.activePetIndex.clamp(0, pets.length - 1);
      final profileStats = await _fetchProfileStats(repository);
      state = state.copyWith(
        pets: pets,
        activePetIndex: clampedIndex,
        profileStats: profileStats,
      );
    } catch (error) {
      state = state.copyWith(error: ApiException.from(error).message);
    }
  }

  void selectActivePet(int index) {
    final upperBound = state.pets.isEmpty ? 1 : state.pets.length - 1;
    if (index < 0 || index > upperBound || index == state.activePetIndex) {
      return;
    }
    state = state.copyWith(activePetIndex: index);
  }

  Future<void> markChatRead(String chatId) async {
    if (!state.readChatIds.contains(chatId)) {
      state = state.copyWith(readChatIds: {...state.readChatIds, chatId});
    }
    final messages = state.messagesFor(chatId);
    final repository = _repository;
    if (repository != null && messages.isNotEmpty) {
      try {
        await repository.markRead(chatId, messages.last.id);
      } catch (_) {
        // Read state is best-effort and will reconcile on the next refresh.
      }
    }
  }

  Future<void> reactToCandidate({
    required bool interested,
    bool superLike = false,
  }) async {
    final repository = _repository;
    if (state.candidateIndex >= state.candidates.length) return;
    final current = state.candidates[state.candidateIndex];
    state = state.copyWith(
      candidateIndex: state.candidateIndex + 1,
      clearLastMatch: true,
      superLikedPetIds: superLike
          ? {...state.superLikedPetIds, current.id}
          : state.superLikedPetIds,
    );

    if (repository == null) {
      if (interested && current.id == 'coco') {
        state = state.copyWith(lastMatch: current);
      }
      return;
    }
    if (state.pets.isEmpty) return;
    try {
      final activeIndex = state.activePetIndex < state.pets.length
          ? state.activePetIndex
          : 0;
      final outcome = await repository.swipe(
        sourcePetId: state.pets[activeIndex].id,
        target: current,
        decision: interested ? (superLike ? 'super_like' : 'like') : 'skip',
      );
      if (outcome.matchedPet != null) {
        state = state.copyWith(
          lastMatch: outcome.matchedPet,
          lastMatchChatId: outcome.chatId,
        );
      }
    } catch (error) {
      state = state.copyWith(error: ApiException.from(error).message);
    }
  }

  void resetMatchDeck() {
    if (_repository == null) {
      state = state.copyWith(candidateIndex: 0, clearLastMatch: true);
    } else {
      initialize(force: true);
    }
  }

  /// Applies a new discovery radius from `_DiscoveryFiltersSheet` — see
  /// server/internal/modules/matching/handler.go's `max_distance_km` query
  /// param (0 means unbounded there, so this UI never lets the slider go
  /// below its 2km floor to avoid accidentally re-introducing that). A
  /// full re-`initialize` is used rather than a narrower candidates-only
  /// fetch to match [resetMatchDeck]'s existing "force full reload"
  /// pattern for any filter/deck change.
  Future<void> setMatchRadiusKm(double radiusKm) async {
    if (radiusKm == state.matchRadiusKm) return;
    state = state.copyWith(matchRadiusKm: radiusKm, candidateIndex: 0);
    if (_repository != null) {
      await initialize(force: true);
    }
  }

  void dismissMatch() {
    state = state.copyWith(clearLastMatch: true);
  }

  Future<void> loadChat(String chatId) async {
    final repository = _repository;
    if (repository == null || state.messagesByChat.containsKey(chatId)) return;
    try {
      final messages = await repository.messages(chatId);
      state = state.copyWith(
        messagesByChat: {...state.messagesByChat, chatId: messages},
      );
    } catch (error) {
      state = state.copyWith(error: ApiException.from(error).message);
    }
  }

  Future<void> sendMessage(String chatId, String value) async {
    final repository = _repository;
    final text = value.trim();
    if (text.isEmpty) return;
    if (repository == null) {
      final message = ChatMessage(
        id: 'local-${DateTime.now().microsecondsSinceEpoch}-${_messageSequence++}',
        text: text,
        time: 'Now',
        isMine: true,
      );
      _appendMessage(chatId, message);
      return;
    }
    try {
      final message = await repository.sendMessage(chatId, text);
      _appendMessage(chatId, message);
    } catch (error) {
      state = state.copyWith(error: ApiException.from(error).message);
    }
  }

  void _appendMessage(String chatId, ChatMessage message) {
    final currentMessages = state.messagesFor(chatId);
    if (currentMessages.any((item) => item.id == message.id)) return;
    state = state.copyWith(
      messagesByChat: {
        ...state.messagesByChat,
        chatId: [...currentMessages, message],
      },
    );
  }

  void _handleRealtimeEvent(RealtimeEvent event) {
    final repository = _repository;
    if (repository == null) return;
    if (event.type == 'match.created') {
      unawaited(initialize(force: true));
      return;
    }
    if (event.type != 'message.created') return;
    final chatId = event.data['chat_id']?.toString();
    if (chatId == null || chatId.isEmpty) return;
    final message = repository.messageFromRealtime(event.data);
    _appendMessage(chatId, message);
    state = state.copyWith(
      chats: state.chats
          .map((chat) {
            if (chat.id != chatId) return chat;
            return ChatPreview(
              id: chat.id,
              petName: chat.petName,
              ownerName: chat.ownerName,
              imageAsset: chat.imageAsset,
              lastMessage: message.text,
              time: message.time,
              unreadCount: message.isMine
                  ? chat.unreadCount
                  : chat.unreadCount + 1,
              isOnline: chat.isOnline,
            );
          })
          .toList(growable: false),
    );
  }

  @override
  void dispose() {
    unawaited(_realtimeSubscription?.cancel());
    unawaited(realtime?.stop());
    super.dispose();
  }
}

final socialControllerProvider =
    StateNotifierProvider<SocialController, SocialState>((ref) {
      final user = ref.read(authControllerProvider).user;
      return SocialController(
        repository: ref.watch(socialRepositoryProvider),
        realtime: ref.watch(realtimeClientProvider),
        currentUserId: user?.id,
        initialProfile: user == null
            ? null
            : UserSetupProfile(
                ownerName: user.name,
                city: user.city,
                petName: 'My pet',
                petType: 'Pet',
                interests: const [],
              ),
      );
    });
