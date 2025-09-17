package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/hanpama/protograph/tests/simple/server/grpcproto"
)

type server struct {
	grpcproto.UnimplementedUserServiceServer

	mu            sync.RWMutex
	users         map[string]*grpcproto.UserSource
	usersByEmail  map[string]*grpcproto.UserSource
	organizations map[string]*grpcproto.OrganizationSource
	posts         map[string]*grpcproto.PostSource
	comments      map[string]*grpcproto.CommentSource
	profiles      map[string]*grpcproto.ProfileSource
	profilesByUID map[string]*grpcproto.ProfileSource

	nextID int
}

func newServer() *server {
	s := &server{
		users:         make(map[string]*grpcproto.UserSource),
		usersByEmail:  make(map[string]*grpcproto.UserSource),
		organizations: make(map[string]*grpcproto.OrganizationSource),
		posts:         make(map[string]*grpcproto.PostSource),
		comments:      make(map[string]*grpcproto.CommentSource),
		profiles:      make(map[string]*grpcproto.ProfileSource),
		profilesByUID: make(map[string]*grpcproto.ProfileSource),
		nextID:        1,
	}

	// Seed some initial data
	s.seedData()

	return s
}

func (s *server) seedData() {
	// Create organizations
	org1 := &grpcproto.OrganizationSource{
		Id:          "org-1",
		Name:        "Tech Corp",
		Description: "A technology company",
	}
	s.organizations[org1.Id] = org1

	org2 := &grpcproto.OrganizationSource{
		Id:          "org-2",
		Name:        "Design Studio",
		Description: "Creative design agency",
	}
	s.organizations[org2.Id] = org2

	// Create users
	now := time.Now().Format(time.RFC3339)

	user1 := &grpcproto.UserSource{
		Id:             "user-1",
		Email:          "john@example.com",
		Name:           "John Doe",
		Age:            30,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
		OrganizationId: "org-1",
	}
	s.users[user1.Id] = user1
	s.usersByEmail[user1.Email] = user1

	user2 := &grpcproto.UserSource{
		Id:             "user-2",
		Email:          "jane@example.com",
		Name:           "Jane Smith",
		Age:            28,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
		OrganizationId: "org-1",
	}
	s.users[user2.Id] = user2
	s.usersByEmail[user2.Email] = user2

	user3 := &grpcproto.UserSource{
		Id:             "user-3",
		Email:          "bob@example.com",
		Name:           "Bob Johnson",
		Age:            35,
		IsActive:       false,
		CreatedAt:      now,
		UpdatedAt:      now,
		OrganizationId: "org-2",
	}
	s.users[user3.Id] = user3
	s.usersByEmail[user3.Email] = user3

	// Create profiles
	profile1 := &grpcproto.ProfileSource{
		Id:        "profile-1",
		UserId:    "user-1",
		Bio:       "Software engineer with passion for Go",
		AvatarUrl: "https://example.com/avatar/john.jpg",
	}
	s.profiles[profile1.Id] = profile1
	s.profilesByUID[profile1.UserId] = profile1

	profile2 := &grpcproto.ProfileSource{
		Id:        "profile-2",
		UserId:    "user-2",
		Bio:       "Full-stack developer",
		AvatarUrl: "https://example.com/avatar/jane.jpg",
	}
	s.profiles[profile2.Id] = profile2
	s.profilesByUID[profile2.UserId] = profile2

	// Create posts
	post1 := &grpcproto.PostSource{
		Id:        "post-1",
		Title:     "Getting Started with Go",
		Content:   "Go is a statically typed, compiled programming language...",
		Published: true,
		AuthorId:  "user-1",
	}
	s.posts[post1.Id] = post1

	post2 := &grpcproto.PostSource{
		Id:        "post-2",
		Title:     "GraphQL Best Practices",
		Content:   "When designing GraphQL APIs, consider these best practices...",
		Published: true,
		AuthorId:  "user-2",
	}
	s.posts[post2.Id] = post2

	post3 := &grpcproto.PostSource{
		Id:        "post-3",
		Title:     "Draft Post",
		Content:   "This is a draft post...",
		Published: false,
		AuthorId:  "user-1",
	}
	s.posts[post3.Id] = post3

	// Create comments
	comment1 := &grpcproto.CommentSource{
		Id:       "comment-1",
		Content:  "Great article!",
		PostId:   "post-1",
		AuthorId: "user-2",
	}
	s.comments[comment1.Id] = comment1

	comment2 := &grpcproto.CommentSource{
		Id:       "comment-2",
		Content:  "Very helpful, thanks!",
		PostId:   "post-1",
		AuthorId: "user-3",
	}
	s.comments[comment2.Id] = comment2

	comment3 := &grpcproto.CommentSource{
		Id:       "comment-3",
		Content:  "I disagree with some points...",
		PostId:   "post-2",
		AuthorId: "user-1",
	}
	s.comments[comment3.Id] = comment3
}

func (s *server) generateID(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%d", prefix, s.nextID)
}

// Query resolvers
func (s *server) ResolveQueryUser(ctx context.Context, req *grpcproto.ResolveQueryUserRequest) (*grpcproto.ResolveQueryUserResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[req.Id]
	if !exists {
		return &grpcproto.ResolveQueryUserResponse{}, nil
	}

	return &grpcproto.ResolveQueryUserResponse{
		Data: user,
	}, nil
}

func (s *server) ResolveQueryUsers(ctx context.Context, req *grpcproto.ResolveQueryUsersRequest) (*grpcproto.ResolveQueryUsersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var users []*grpcproto.UserSource
	for _, user := range s.users {
		users = append(users, user)
	}

	return &grpcproto.ResolveQueryUsersResponse{
		Data: users,
	}, nil
}

func (s *server) ResolveQueryNode(ctx context.Context, req *grpcproto.ResolveQueryNodeRequest) (*grpcproto.ResolveQueryNodeResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return &grpcproto.ResolveQueryNodeResponse{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if user, exists := s.users[id]; exists {
		return nodeResponse("User", user)
	}
	if org, exists := s.organizations[id]; exists {
		return nodeResponse("Organization", org)
	}
	if post, exists := s.posts[id]; exists {
		return nodeResponse("Post", post)
	}
	if comment, exists := s.comments[id]; exists {
		return nodeResponse("Comment", comment)
	}
	if profile, exists := s.profiles[id]; exists {
		return nodeResponse("Profile", profile)
	}

	return &grpcproto.ResolveQueryNodeResponse{}, nil
}

func (s *server) ResolveQuerySearch(ctx context.Context, req *grpcproto.ResolveQuerySearchRequest) (*grpcproto.ResolveQuerySearchResponse, error) {
	term := strings.TrimSpace(req.GetTerm())
	if term == "" {
		return &grpcproto.ResolveQuerySearchResponse{}, nil
	}

	needle := strings.ToLower(term)

	s.mu.RLock()
	defer s.mu.RUnlock()

	contains := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), needle) {
				return true
			}
		}
		return false
	}

	var results []*grpcproto.SearchResultSource

	userKeys := make([]string, 0, len(s.users))
	for id := range s.users {
		userKeys = append(userKeys, id)
	}
	sort.Strings(userKeys)
	for _, id := range userKeys {
		user := s.users[id]
		if contains(user.Name, user.Email) {
			results = append(results, &grpcproto.SearchResultSource{Value: &grpcproto.SearchResultSource_User{User: user}})
		}
	}

	orgKeys := make([]string, 0, len(s.organizations))
	for id := range s.organizations {
		orgKeys = append(orgKeys, id)
	}
	sort.Strings(orgKeys)
	for _, id := range orgKeys {
		org := s.organizations[id]
		if contains(org.Name, org.Description) {
			results = append(results, &grpcproto.SearchResultSource{Value: &grpcproto.SearchResultSource_Organization{Organization: org}})
		}
	}

	postKeys := make([]string, 0, len(s.posts))
	for id := range s.posts {
		postKeys = append(postKeys, id)
	}
	sort.Strings(postKeys)
	for _, id := range postKeys {
		post := s.posts[id]
		if contains(post.Title, post.Content) {
			results = append(results, &grpcproto.SearchResultSource{Value: &grpcproto.SearchResultSource_Post{Post: post}})
		}
	}

	return &grpcproto.ResolveQuerySearchResponse{
		Data: results,
	}, nil
}

func nodeResponse(typename string, msg proto.Message) (*grpcproto.ResolveQueryNodeResponse, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal %s payload: %v", typename, err)
	}

	return &grpcproto.ResolveQueryNodeResponse{
		Data: &grpcproto.NodeSource{
			Typename: typename,
			Payload:  payload,
		},
	}, nil
}

// Mutation resolvers
func (s *server) ResolveMutationCreateUser(ctx context.Context, req *grpcproto.ResolveMutationCreateUserRequest) (*grpcproto.ResolveMutationCreateUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Input == nil {
		return nil, status.Error(codes.InvalidArgument, "input is required")
	}

	// Check if email already exists
	if _, exists := s.usersByEmail[req.Input.Email]; exists {
		return nil, status.Error(codes.AlreadyExists, "user with this email already exists")
	}

	now := time.Now().Format(time.RFC3339)
	user := &grpcproto.UserSource{
		Id:             s.generateID("user"),
		Email:          req.Input.Email,
		Name:           req.Input.Name,
		Age:            req.Input.Age,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
		OrganizationId: req.Input.OrganizationId,
	}

	s.users[user.Id] = user
	s.usersByEmail[user.Email] = user

	return &grpcproto.ResolveMutationCreateUserResponse{
		Data: user,
	}, nil
}

func (s *server) ResolveMutationUpdateUser(ctx context.Context, req *grpcproto.ResolveMutationUpdateUserRequest) (*grpcproto.ResolveMutationUpdateUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[req.Id]
	if !exists {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	if req.Input == nil {
		return nil, status.Error(codes.InvalidArgument, "input is required")
	}

	// Create a copy to modify
	updatedUser := &grpcproto.UserSource{
		Id:             user.Id,
		Email:          user.Email,
		Name:           user.Name,
		Age:            user.Age,
		IsActive:       user.IsActive,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      time.Now().Format(time.RFC3339),
		OrganizationId: user.OrganizationId,
	}

	// Apply updates
	if req.Input.Email != "" && req.Input.Email != user.Email {
		// Check if new email already exists
		if _, exists := s.usersByEmail[req.Input.Email]; exists {
			return nil, status.Error(codes.AlreadyExists, "user with this email already exists")
		}
		// Remove old email mapping
		delete(s.usersByEmail, user.Email)
		updatedUser.Email = req.Input.Email
		s.usersByEmail[updatedUser.Email] = updatedUser
	}

	if req.Input.Name != "" {
		updatedUser.Name = req.Input.Name
	}

	if req.Input.Age != 0 {
		updatedUser.Age = req.Input.Age
	}

	// IsActive is a bool in proto3, we need to track if it was set
	// For simplicity, we'll update it if it's different from current value
	if req.Input.IsActive != user.IsActive {
		updatedUser.IsActive = req.Input.IsActive
	}

	s.users[updatedUser.Id] = updatedUser

	return &grpcproto.ResolveMutationUpdateUserResponse{
		Data: updatedUser,
	}, nil
}

func (s *server) ResolveMutationDeleteUser(ctx context.Context, req *grpcproto.ResolveMutationDeleteUserRequest) (*grpcproto.ResolveMutationDeleteUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[req.Id]
	if !exists {
		return &grpcproto.ResolveMutationDeleteUserResponse{
			Data: false,
		}, nil
	}

	delete(s.users, req.Id)
	delete(s.usersByEmail, user.Email)

	// Delete associated profile if exists
	if profile, exists := s.profilesByUID[req.Id]; exists {
		delete(s.profiles, profile.Id)
		delete(s.profilesByUID, req.Id)
	}

	return &grpcproto.ResolveMutationDeleteUserResponse{
		Data: true,
	}, nil
}

// Field resolvers
func (s *server) ResolveUserPosts(ctx context.Context, req *grpcproto.ResolveUserPostsRequest) (*grpcproto.ResolveUserPostsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var posts []*grpcproto.PostSource
	for _, post := range s.posts {
		if post.AuthorId == req.AuthorId {
			posts = append(posts, post)
		}
	}

	return &grpcproto.ResolveUserPostsResponse{
		Data: posts,
	}, nil
}

func (s *server) ResolveOrganizationMemberCount(ctx context.Context, req *grpcproto.ResolveOrganizationMemberCountRequest) (*grpcproto.ResolveOrganizationMemberCountResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := int32(0)
	for _, user := range s.users {
		if user.OrganizationId == req.Id {
			count++
		}
	}

	return &grpcproto.ResolveOrganizationMemberCountResponse{
		Data: count,
	}, nil
}

func (s *server) ResolveOrganizationMembers(ctx context.Context, req *grpcproto.ResolveOrganizationMembersRequest) (*grpcproto.ResolveOrganizationMembersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var members []*grpcproto.UserSource
	for _, user := range s.users {
		if user.OrganizationId == req.OrganizationId {
			members = append(members, user)
		}
	}

	return &grpcproto.ResolveOrganizationMembersResponse{
		Data: members,
	}, nil
}

func (s *server) ResolvePostComments(ctx context.Context, req *grpcproto.ResolvePostCommentsRequest) (*grpcproto.ResolvePostCommentsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var comments []*grpcproto.CommentSource
	for _, comment := range s.comments {
		if comment.PostId == req.PostId {
			comments = append(comments, comment)
		}
	}

	return &grpcproto.ResolvePostCommentsResponse{
		Data: comments,
	}, nil
}

// Batch loaders
func (s *server) BatchLoadUserById(ctx context.Context, req *grpcproto.BatchLoadUserByIdRequest) (*grpcproto.BatchLoadUserByIdResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &grpcproto.BatchLoadUserByIdResponse{
		Batches: make([]*grpcproto.LoadUserByIdResponse, len(req.Batches)),
	}

	for i, r := range req.Batches {
		if user, exists := s.users[r.Id]; exists {
			response.Batches[i] = &grpcproto.LoadUserByIdResponse{
				Data: user,
			}
		} else {
			response.Batches[i] = &grpcproto.LoadUserByIdResponse{}
		}
	}

	return response, nil
}

func (s *server) BatchLoadUserByEmail(ctx context.Context, req *grpcproto.BatchLoadUserByEmailRequest) (*grpcproto.BatchLoadUserByEmailResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &grpcproto.BatchLoadUserByEmailResponse{
		Batches: make([]*grpcproto.LoadUserByEmailResponse, len(req.Batches)),
	}

	for i, r := range req.Batches {
		if user, exists := s.usersByEmail[r.Email]; exists {
			response.Batches[i] = &grpcproto.LoadUserByEmailResponse{
				Data: user,
			}
		} else {
			response.Batches[i] = &grpcproto.LoadUserByEmailResponse{}
		}
	}

	return response, nil
}

func (s *server) BatchLoadOrganizationById(ctx context.Context, req *grpcproto.BatchLoadOrganizationByIdRequest) (*grpcproto.BatchLoadOrganizationByIdResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &grpcproto.BatchLoadOrganizationByIdResponse{
		Batches: make([]*grpcproto.LoadOrganizationByIdResponse, len(req.Batches)),
	}

	for i, r := range req.Batches {
		if org, exists := s.organizations[r.Id]; exists {
			response.Batches[i] = &grpcproto.LoadOrganizationByIdResponse{
				Data: org,
			}
		} else {
			response.Batches[i] = &grpcproto.LoadOrganizationByIdResponse{}
		}
	}

	return response, nil
}

func (s *server) BatchLoadPostById(ctx context.Context, req *grpcproto.BatchLoadPostByIdRequest) (*grpcproto.BatchLoadPostByIdResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &grpcproto.BatchLoadPostByIdResponse{
		Batches: make([]*grpcproto.LoadPostByIdResponse, len(req.Batches)),
	}

	for i, r := range req.Batches {
		if post, exists := s.posts[r.Id]; exists {
			response.Batches[i] = &grpcproto.LoadPostByIdResponse{
				Data: post,
			}
		} else {
			response.Batches[i] = &grpcproto.LoadPostByIdResponse{}
		}
	}

	return response, nil
}

func (s *server) BatchLoadCommentById(ctx context.Context, req *grpcproto.BatchLoadCommentByIdRequest) (*grpcproto.BatchLoadCommentByIdResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &grpcproto.BatchLoadCommentByIdResponse{
		Batches: make([]*grpcproto.LoadCommentByIdResponse, len(req.Batches)),
	}

	for i, r := range req.Batches {
		if comment, exists := s.comments[r.Id]; exists {
			response.Batches[i] = &grpcproto.LoadCommentByIdResponse{
				Data: comment,
			}
		} else {
			response.Batches[i] = &grpcproto.LoadCommentByIdResponse{}
		}
	}

	return response, nil
}

func (s *server) BatchLoadProfileByUserId(ctx context.Context, req *grpcproto.BatchLoadProfileByUserIdRequest) (*grpcproto.BatchLoadProfileByUserIdResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &grpcproto.BatchLoadProfileByUserIdResponse{
		Batches: make([]*grpcproto.LoadProfileByUserIdResponse, len(req.Batches)),
	}

	for i, r := range req.Batches {
		if profile, exists := s.profilesByUID[r.UserId]; exists {
			response.Batches[i] = &grpcproto.LoadProfileByUserIdResponse{
				Data: profile,
			}
		} else {
			response.Batches[i] = &grpcproto.LoadProfileByUserIdResponse{}
		}
	}

	return response, nil
}

func main() {
	addr := flag.String("addr", ":50051", "the address to listen on")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *addr, err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(loggingUnaryServerInterceptor),
	)
	grpcproto.RegisterUserServiceServer(s, newServer())

	log.Printf("gRPC server starting on %s", *addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// loggingUnaryServerInterceptor logs exactly one line per unary RPC with method, duration, and compact JSON for req/resp (or error).
func loggingUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	reqJSON := toCompactJSON(req)

	resp, err := handler(ctx, req)
	dur := time.Since(start)

	if err != nil {
		st, _ := status.FromError(err)
		log.Printf("grpc method=%s code=%s duration=%s req=%s error=%q", info.FullMethod, st.Code(), dur, reqJSON, st.Message())
		return resp, err
	}

	respJSON := toCompactJSON(resp)
	log.Printf("grpc method=%s duration=%s req=%s resp=%s", info.FullMethod, dur, reqJSON, respJSON)
	return resp, nil
}

// toCompactJSON marshals a protobuf message to a single-line JSON string; falls back to type name if not proto or on error.
func toCompactJSON(msg interface{}) string {
	if m, ok := msg.(proto.Message); ok {
		b, err := protojson.MarshalOptions{EmitUnpopulated: true, UseEnumNumbers: false}.Marshal(m)
		if err == nil {
			return string(b)
		}
	}
	return fmt.Sprintf("\"%T\"", msg)
}
